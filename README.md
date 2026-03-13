# GoMCTools


![Go Badge](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=fff&style=flat) ![Arch Linux Badge](https://img.shields.io/badge/Arch%20Linux-1793D1?logo=archlinux&logoColor=fff&style=flat)
[![Cash App Badge](https://img.shields.io/badge/Cash%20App-00C244?logo=cashapp&logoColor=fff&style=flat)](https://cash.app/$ItzDavL)
****

**GoMCTools** is a terminal-based utility (TUI) designed to streamline the workflow for Minecraft modpack developers. Built specifically for **Prism Launcher** instances, it automates tedious tasks like generating modlists, managing pack configurations, and preparing releases.

## Features

### 📦 Instance Selector
- Browse and load **Prism Launcher** instances directly from the terminal.
- Auto-detects `mmc-pack.json` to parse mod metadata (Loader, MC Version, etc.).
- Remembers your last loaded pack for quick access on startup.

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
| `Tab` | Next Page |
| `Shift+Tab` | Previous Page |
| `?` | Toggle Help Overlay |
| `q` / `Ctrl+c` | Quit |


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
- Bubble Tea
- Bubbles
- Lipgloss
- Glamour
- Bubblezone

***
*Created by ItzDabbzz*

