---
title: Maintaining Architecture Diagrams & Docs
description: Contributor workflow for modifying, compiling, and testing Graphviz architecture diagrams and documentation.
---

This guide explains how contributors and team members can maintain, update, and compile the architecture documentation and Graphviz diagrams.

---

## Directory Conventions

```text
homectl/
├── scripts/
│   └── build-diagrams.sh              # Automated compilation script
├── docs/
│   ├── src/
│   │   ├── assets/
│   │   │   └── architecture/          # Source .dot files and compiled .webp images
│   │   │       ├── homectl-overall.dot
│   │   │       ├── homectl-overall.webp
│   │   │       ├── sonos.dot
│   │   │       ├── sonos.webp
│   │   │       ├── lutron.dot
│   │   │       ├── lutron.webp
│   │   │       ├── qolsys.dot
│   │   │       ├── qolsys.webp
│   │   │       ├── cast.dot
│   │   │       ├── cast.webp
│   │   │       ├── camera.dot
│   │   │       └── camera.webp
│   │   └── content/
│   │       └── docs/
│   │           └── architecture/      # Starlight Markdown documentation pages
```

---

## Prerequisites

To edit and compile diagrams, install **Graphviz**:

```bash
# macOS (Homebrew)
brew install graphviz

# Debian / Ubuntu Linux
sudo apt-get install -y graphviz
```

Verify `dot` is available on your PATH:
```bash
dot -v
```

---

## Updating an Architecture Diagram

### 1. Edit the Source `.dot` File
Edit the corresponding `.dot` file in `docs/src/assets/architecture/`.

#### Graphviz Styling Guidelines:
* **Typography:** Use clean system fonts:
  ```text
  fontname="Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
  ```
* **HTML Labels:** Use HTML-like tables for multi-line nodes with subtitles:
  ```text
  node_name [
    label=<<table border="0" cellborder="0" cellspacing="2">
      <tr><td><font color="#0284c7"><b>Component Name</b></font></td></tr>
      <tr><td><font color="#64748b" point-size="8">Protocol or Detail</font></td></tr>
    </table>>,
    fillcolor="#ffffff",
    color="#0284c7"
  ];
  ```
* **Color Palette:**
  * **Interactions / Clients:** Blue (`#0284c7`)
  * **Agent Plugins & Skills:** Purple (`#7c3aed`)
  * **Gateways & Servers:** Green (`#059669`)
  * **Go Modules & Core:** Amber (`#d97706`)
  * **Physical Hardware:** Red / Orange (`#dc2626` / `#ea580c`)

---

### 2. Compile Diagrams to WebP

Run the automated build target from the repository root:

```bash
make diagrams
```

Or execute the build script directly:
```bash
./scripts/build-diagrams.sh
```

#### What the script does:
1. Iterates over every `.dot` file in `docs/src/assets/architecture/`.
2. Runs `dot -Twebp "$dotfile" -o "${dotfile%.dot}.webp"`.
3. Automatically mirrors service diagrams to their respective plugin asset folders (e.g., `plugins/sonos/assets/architecture.webp`).
4. Prints output file sizes for inspection.

---

### 3. Verify the Starlight Documentation Site

After recompiling diagrams or updating Markdown pages, build the documentation site locally:

```bash
npm --prefix docs run build
```

Or start the live development preview:
```bash
npm --prefix docs run dev
```

---

### 4. Git & Commit Guidelines

* **Commit `.dot` and `.webp` together:** Always commit both the source Graphviz `.dot` file and the generated `.webp` image in the same commit.
* **Intermediate PNGs are gitignored:** Intermediate `.png` files are ignored via `.gitignore` to avoid repository bloat.
* **Conventional Commits:** Use `docs(architecture): update sonos vertical with favorites flow`.
