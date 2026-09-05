// Package main provides the sync-skills tool to synchronize canonical skills
// from top-level skills/ into self-contained plugins/<svc>/skills/ bundles.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type BundleConfig struct {
	Skills []string `json:"skills"`
}

type PluginConfig struct {
	Skills []string `json:"skills,omitempty"`
}

var scriptRefRegex = regexp.MustCompile(`scripts/[a-zA-Z0-9_\-\./]+\.(py|sh|ts|js|go)`)

func main() {
	rootDir := flag.String("root", ".", "Path to repository root")
	checkMode := flag.Bool("check", false, "Check if plugin skills are in sync with canonical skills without writing")
	verbose := flag.Bool("verbose", false, "Print detailed synchronization logs")
	flag.Parse()

	absRoot, err := filepath.Abs(*rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root directory: %v\n", err)
		os.Exit(1)
	}

	skillsDir := filepath.Join(absRoot, "skills")
	pluginsDir := filepath.Join(absRoot, "plugins")

	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		fmt.Printf("No skills/ directory found at %s; nothing to sync.\n", skillsDir)
		os.Exit(0)
	}
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		fmt.Printf("No plugins/ directory found at %s; nothing to sync.\n", pluginsDir)
		os.Exit(0)
	}

	pluginEntries, err := os.ReadDir(pluginsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading plugins directory: %v\n", err)
		os.Exit(1)
	}

	hasErrors := false

	for _, pEntry := range pluginEntries {
		if !pEntry.IsDir() {
			continue
		}
		pluginName := pEntry.Name()
		pluginPath := filepath.Join(pluginsDir, pluginName)

		skillNames := resolvePluginSkills(pluginPath)
		if len(skillNames) == 0 {
			continue
		}

		for _, skillName := range skillNames {
			srcSkillDir := filepath.Join(skillsDir, skillName)
			dstSkillDir := filepath.Join(pluginPath, "skills", skillName)

			if _, err := os.Stat(srcSkillDir); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "ERROR: Plugin %q references skill %q, but %s does not exist\n",
					pluginName, skillName, srcSkillDir)
				hasErrors = true
				continue
			}

			if *checkMode {
				if err := checkSkillSync(srcSkillDir, dstSkillDir, pluginName, skillName, *verbose); err != nil {
					fmt.Fprintf(os.Stderr, "OUT-OF-SYNC: %v\n", err)
					hasErrors = true
				} else if *verbose {
					fmt.Printf("OK: %s -> %s/skills/%s is in sync\n", skillName, pluginName, skillName)
				}
			} else {
				if err := copySkillTree(srcSkillDir, dstSkillDir, *verbose); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR copying %s -> %s: %v\n", srcSkillDir, dstSkillDir, err)
					hasErrors = true
				} else {
					fmt.Printf("Synced: skills/%s -> plugins/%s/skills/%s\n", skillName, pluginName, skillName)
				}
			}

			// Validate that any referenced scripts in SKILL.md exist
			targetSkillMD := filepath.Join(dstSkillDir, "SKILL.md")
			if *checkMode {
				targetSkillMD = filepath.Join(dstSkillDir, "SKILL.md")
			}
			if err := validateScriptReferences(targetSkillMD, dstSkillDir); err != nil {
				fmt.Fprintf(os.Stderr, "REFERENCE ERROR in %s: %v\n", targetSkillMD, err)
				hasErrors = true
			}
		}

		// Ensure OpenCode configuration example is generated and verified
		if err := syncOpenCodeConfig(pluginName, pluginPath, *checkMode, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "OPENCODE CONFIG ERROR in %s: %v\n", pluginName, err)
			hasErrors = true
		}
	}

	if hasErrors {
		if *checkMode {
			fmt.Fprintln(os.Stderr, "\nRun 'make sync-skills' (or 'go run ./cmd/sync-skills') to synchronize skills.")
		}
		os.Exit(1)
	}

	if *checkMode {
		fmt.Println("All plugin skills are in sync and self-contained.")
	}
}

// resolvePluginSkills discovers which skills belong to a plugin by inspecting:
// 1. plugins/<p>/bundle.json ("skills": [...])
// 2. plugins/<p>/plugin.json ("skills": [...])
// 3. Any existing directories under plugins/<p>/skills/*
func resolvePluginSkills(pluginPath string) []string {
	skillSet := make(map[string]bool)

	// 1. Check bundle.json
	bundlePath := filepath.Join(pluginPath, "bundle.json")
	if data, err := os.ReadFile(bundlePath); err == nil {
		var cfg BundleConfig
		if json.Unmarshal(data, &cfg) == nil {
			for _, s := range cfg.Skills {
				if strings.TrimSpace(s) != "" {
					skillSet[strings.TrimSpace(s)] = true
				}
			}
		}
	}

	// 2. Check plugin.json
	pluginJsonPath := filepath.Join(pluginPath, "plugin.json")
	if data, err := os.ReadFile(pluginJsonPath); err == nil {
		var cfg PluginConfig
		if json.Unmarshal(data, &cfg) == nil {
			for _, s := range cfg.Skills {
				if strings.TrimSpace(s) != "" {
					skillSet[strings.TrimSpace(s)] = true
				}
			}
		}
	}

	// 3. Check existing skills subdirectory
	pluginSkillsDir := filepath.Join(pluginPath, "skills")
	if entries, err := os.ReadDir(pluginSkillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				skillSet[e.Name()] = true
			}
		}
	}

	var results []string
	for s := range skillSet {
		results = append(results, s)
	}
	return results
}

// copySkillTree mirrors srcDir into dstDir, preserving directory structure and permissions.
func copySkillTree(srcDir, dstDir string, verbose bool) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstDir, rel)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if verbose {
			fmt.Printf("  Copying %s -> %s\n", path, targetPath)
		}

		return os.WriteFile(targetPath, content, info.Mode())
	})
}

// checkSkillSync verifies that dstDir matches srcDir identically.
func checkSkillSync(srcDir, dstDir, pluginName, skillName string, verbose bool) error {
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin %q is missing skill directory %s", pluginName, dstDir)
	}

	// Check that all files in srcDir exist and match in dstDir
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstDir, rel)

		srcData, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading source %s: %w", path, err)
		}

		dstData, err := os.ReadFile(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("file %s is missing in plugin bundle %s", rel, dstDir)
			}
			return fmt.Errorf("reading target %s: %w", targetPath, err)
		}

		if !bytes.Equal(srcData, dstData) {
			return fmt.Errorf("file %s differs between canonical skills/%s and plugin %s", rel, skillName, pluginName)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// validateScriptReferences inspects SKILL.md for referenced scripts and ensures they exist.
func validateScriptReferences(skillMDPath, skillDir string) error {
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil // SKILL.md missing checked elsewhere
	}

	matches := scriptRefRegex.FindAllString(string(data), -1)
	for _, m := range matches {
		expectedPath := filepath.Join(skillDir, m)
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			return fmt.Errorf("SKILL.md references %q, but %s does not exist inside the bundle", m, expectedPath)
		}
	}
	return nil
}

// syncOpenCodeConfig generates or validates the OpenCode-native config example (opencode.jsonc).
func syncOpenCodeConfig(pluginName, pluginPath string, checkMode, verbose bool) error {
	targetPath := filepath.Join(pluginPath, "opencode.jsonc")

	type OpenCodeServer struct {
		Type    string   `json:"type"`
		Command []string `json:"command"`
		Enabled bool     `json:"enabled"`
	}

	type OpenCodeConfig struct {
		Schema string                    `json:"$schema"`
		MCP    map[string]OpenCodeServer `json:"mcp"`
	}

	cfg := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]OpenCodeServer{
			pluginName: {
				Type:    "local",
				Command: []string{fmt.Sprintf("./bin/mcp-%s", pluginName)},
				Enabled: true,
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if checkMode {
		existing, err := os.ReadFile(targetPath)
		if err != nil {
			return fmt.Errorf("missing %s in plugin bundle", targetPath)
		}
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("opencode.jsonc in %s is out of date", pluginPath)
		}
		if verbose {
			fmt.Printf("OK: %s/opencode.jsonc is valid\n", pluginName)
		}
		return nil
	}

	if verbose {
		fmt.Printf("  Writing %s\n", targetPath)
	}
	return os.WriteFile(targetPath, data, 0644)
}
