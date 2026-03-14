# GoMCTools

![Go Badge](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=fff&style=flat) ![Arch Linux Badge](https://img.shields.io/badge/Arch%20Linux-1793D1?logo=archlinux&logoColor=fff&style=flat)
[![Cash App Badge](https://img.shields.io/badge/Cash%20App-00C244?logo=cashapp&logoColor=fff&style=flat)](https://cash.app/$ItzDavL)

---

**GoMCTools** is a terminal-based utility (TUI) designed to streamline the workflow for Minecraft modpack developers. Built for **Prism Launcher** and **CurseForge** instances, it automates tedious tasks like generating modlists, managing pack configurations, and preparing releases.

## Features

### 📦 Instance Selector
- Browse and load **Prism Launcher** and **CurseForge** instances directly from the terminal.
- Auto-detects the pack format — no configuration required:
  - **Prism**: reads `mmc-pack.json` and the `.index` TOML mod entries.
  - **CurseForge**: streams `minecraftinstance.json` (handles 15MB+ files efficiently) or falls back to `manifest.json`.
- Extracts mod metadata including loader, MC version, release type, and download URLs.
- Remembers your last loaded pack for quick access on startup.
- Opt-in directory browser with a focus-aware border — arrow keys are never stolen from the tab-bar until you activate it.

### 📝 Modlist Generator
Create professional Markdown modlists for your documentation or GitHub releases.
- **Live Preview**: See how your modlist looks directly in the terminal (rendered with Glamour).
- **Flexible Layouts**:
  - **Merged**: A single alphabetical list of all mods.
  - **Split**: Separate sections for Client-side, Server-side, and Shared mods.
- **Customizable Metadata**: Toggle links, side tags, source (Modrinth/CurseForge), game versions, and filenames.
- **Export**: Copy markdown to clipboard or save to `modlist.md` in the instance folder.

### ⚙️ Configurable
- Persistent settings via `~/.config/gomctools/config.toml`.
- Toggleable auto-load behavior.
- Full keyboard and mouse support.

## Installation

### From Source
Ensure you have **Go 1.21+** installed.

```bash
git clone https://github.com/ItzDabbzz/GoMCTools.git
cd GoMCTools
go build -o gomctools .
./gomctools
```

## Usage

### Global Controls
| Key | Action |
| :--- | :--- |
| `Tab` / `Shift+Tab` | Next / Previous page |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

### Selector Page
| Key | Context | Action |
| :--- | :--- | :--- |
| `Enter` | Text input focused | Load pack from typed path |
| `F` / `Tab` | Browser not focused | Activate the directory browser |
| `↑` / `↓` | Browser focused | Navigate files/directories |
| `l` / `→` | Browser focused | Open / descend into directory |
| `Enter` | Browser focused | Load the current directory as pack |
| `Esc` | Browser focused | Exit browser, return arrow keys to tab-bar |
| `?` | Anywhere | Toggle help overlay |
| `q` / `Ctrl+C` | Anywhere | Quit |

### Modlist Generator
| Key | Action |
| :--- | :--- |
| `1` | Switch to **Merged** layout |
| `2` | Switch to **Split by Side** layout |
| `a` | Toggle **Links** (Modrinth/CurseForge/URL) |
| `i` | Toggle **Side** tags (Client/Server) |
| `o` | Toggle **Source** tags |
| `c` | **Copy** Markdown to clipboard |
| `e` | **Export** to `modlist.md` |

## Supported Pack Formats

| Format | Detection File | Mod Metadata |
| :--- | :--- | :--- |
| Prism Launcher | `mmc-pack.json` | Full — name, side, loader, source, hashes |
| CurseForge (full) | `minecraftinstance.json` | Full — name, release type, game versions, download URL |
| CurseForge (export) | `manifest.json` | Partial — project/file IDs only, no mod names |

> CurseForge's `manifest.json`-only mode is a fallback for exported zips that haven't been opened in the CurseForge app. Mod names will display as `Project <ID>` until a full instance file is available.

## Configuration

Configuration is stored in `~/.config/gomctools/config.toml`.

```toml
auto_load_previous_state = true

[modlist]
mode = 0             # 0 = Merged, 1 = Split
attach_links = true
include_side = true
```

## Credits & Libraries

Built with the Charm stack:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - A powerful little TUI framework 🏗
- [Bubbles](https://charm.land/bubbles/v2) - TUI Components for Bubble Tea 🫧
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for nice terminal layouts 👄
- [Glamour](https://charm.land/glamour/v2) - Stylesheet-based markdown rendering for your CLI apps 💇🏻‍♀️
- [Bubblezone](https://github.com/lrstanley/bubblezone) - BubbleTea mouse event tracking utility

---

*Created by ItzDabbzz* (First go project, kinda nevrous 👉👈)