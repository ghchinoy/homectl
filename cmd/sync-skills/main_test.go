package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCopyAndCheckSkillSync(t *testing.T) {
	tmp := t.TempDir()
	srcSkill := filepath.Join(tmp, "skills", "sonos-soundscape")
	dstSkill := filepath.Join(tmp, "plugins", "sonos", "skills", "sonos-soundscape")

	if err := os.MkdirAll(filepath.Join(srcSkill, "scripts"), 0755); err != nil {
		t.Fatalf("os.MkdirAll(srcSkill) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcSkill, "SKILL.md"), []byte("# Skill\nRun `scripts/helper.sh`.\n"), 0644); err != nil {
		t.Fatalf("os.WriteFile(SKILL.md) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcSkill, "scripts", "helper.sh"), []byte("#!/bin/sh\necho ok\n"), 0755); err != nil {
		t.Fatalf("os.WriteFile(helper.sh) failed: %v", err)
	}

	// Missing destination directory should fail checkSkillSync
	if err := checkSkillSync(srcSkill, dstSkill, "sonos", "sonos-soundscape", false); err == nil {
		t.Fatal("checkSkillSync(missing dst) = nil, want error")
	}

	// Copy and verify sync passes
	if err := copySkillTree(srcSkill, dstSkill, false); err != nil {
		t.Fatalf("copySkillTree(%q, %q) failed: %v", srcSkill, dstSkill, err)
	}
	if err := checkSkillSync(srcSkill, dstSkill, "sonos", "sonos-soundscape", false); err != nil {
		t.Errorf("checkSkillSync(synced) = %v, want nil", err)
	}

	// Validate script reference passes when script exists
	if err := validateScriptReferences(filepath.Join(dstSkill, "SKILL.md"), dstSkill); err != nil {
		t.Errorf("validateScriptReferences(valid) = %v, want nil", err)
	}

	// Mutate destination file to trigger drift detection
	_ = os.WriteFile(filepath.Join(dstSkill, "SKILL.md"), []byte("# Drifted\nRun `scripts/missing.sh`.\n"), 0644)
	if err := checkSkillSync(srcSkill, dstSkill, "sonos", "sonos-soundscape", false); err == nil {
		t.Error("checkSkillSync(drifted) = nil, want drift error")
	}
	if err := validateScriptReferences(filepath.Join(dstSkill, "SKILL.md"), dstSkill); err == nil {
		t.Error("validateScriptReferences(missing.sh) = nil, want missing script error")
	}
}

func TestResolvePluginSkillsAndOpenCodeConfig(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "sonos")
	if err := os.MkdirAll(filepath.Join(pluginDir, "skills", "existing-skill"), 0755); err != nil {
		t.Fatalf("os.MkdirAll(pluginDir) failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(pluginDir, "bundle.json"), []byte(`{"skills":["bundle-skill"]}`), 0644)
	_ = os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"skills":["plugin-skill"]}`), 0644)

	skills := resolvePluginSkills(pluginDir)
	slices.Sort(skills)
	want := []string{"bundle-skill", "existing-skill", "plugin-skill"}
	if !slices.Equal(skills, want) {
		t.Errorf("resolvePluginSkills(%q) = %v, want %v", pluginDir, skills, want)
	}

	// Missing opencode.jsonc fails in checkMode
	if err := syncOpenCodeConfig("sonos", pluginDir, true, false); err == nil {
		t.Fatal("syncOpenCodeConfig(missing, checkMode=true) = nil, want error")
	}

	// Write opencode.jsonc and verify checkMode succeeds
	if err := syncOpenCodeConfig("sonos", pluginDir, false, false); err != nil {
		t.Fatalf("syncOpenCodeConfig(write) failed: %v", err)
	}
	if err := syncOpenCodeConfig("sonos", pluginDir, true, false); err != nil {
		t.Errorf("syncOpenCodeConfig(valid, checkMode=true) = %v, want nil", err)
	}
}
