# GoMCTools

![Go Badge](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=fff&style=flat) 


---

**GoMCTools** is a terminal-based utility (TUI) designed to streamline the workflow for Minecraft modpack developers. Built for **Prism Launcher** and **CurseForge** instances, it automates tedious tasks like generating modlists, managing pack configurations, and preparing releases.

## Features

### � Dashboard
- Quick overview of the loaded pack.
- Displays pack metadata: Author, Version, Minecraft Version, and Loader.
- **Mod Analytics**: Breakdown of mod sources (Modrinth vs CurseForge) with a visual distribution bar.
- Tracks total mod counts and identifies "Unknown" sources.

### �📦 Instance Selector
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
- **Multiple Formats**: Export as Markdown Bullets, GFM Tables, or BBCode for forums.
- **Customizable Metadata**: Toggle links, side tags, source, game versions, and filenames.
- **Smart Sorting**: Sort by Name, Source, or Side with toggleable ascending/descending order.
- **Export**: Copy markdown to clipboard or save to `modlist.md` in the instance folder.
- **Raw Preview**: Toggle between rendered Glamour output and the raw source text.

### 🧹 Pack Cleaner
- Keep your instance clean by selectively removing generated files and folders (logs, cache, etc.).
- **Presets**: Comes with sensible defaults for common Minecraft bloat.
- **Custom Rules**: Add, edit, or delete your own custom cleaning patterns via the TUI.
- **Safety First**: Refuses to delete files outside the instance root.
- **Persistence**: Saves custom rules to `gomctools.cleaner.json` within the instance for portability.

### ⚙️ Configurable
- Persistent settings via `~/.config/gomctools/config.toml`.
- Toggleable auto-load behavior.
- Confirmation modals for destructive actions (like resetting settings).
- Full keyboard and mouse support.

## Screenshots

### Home
<!-- !Home -->
![Home](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/home.png)
*Main entry point with quick-start controls.*

### Selector
<!-- !Selector -->
![Selector No Pack](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/selector_nopack.png)
![Selector Pack](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/selector_pack.png)
*Instance discovery and directory browsing.*

### Dashboard
<!-- !Dashboard -->
![Dashboard No Pack](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/dash_nopack.png)
![Dashboard Pack](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/dash_pack.png)
![Dashboard Multi-Source Pack](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/dash_pack_multi.png)
*Pack metadata and mod distribution analytics.*

### Modlist Generator
<!-- !Modlist -->
![Modlist No Pack](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/modlist_nopack.png)
![Modlist Pack](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/modlist_pack.png)
*Dynamic preview and configuration for documentation.*

### Pack Cleaner
<!-- !Cleaner -->
![Pack Cleaner](https://raw.githubusercontent.com/ItzDabbzz/GoMCTools/refs/heads/main/docs/images/packcleaner_nopack.png)
*Managing custom cleaning presets and execution.*

## Installation

### From Source
Ensure you have **Go 1.21+** installed.

```bash
go install github.com/ItzDabbzz/GoMCTools@latest
```

Or manually:

```bash
git clone https://github.com/ItzDabbzz/GoMCTools.git && cd GoMCTools
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

GoMCTools manages persistent settings via a TOML configuration file.

> [!IMPORTANT]
> Manual edits to `config.toml` should only be made while the application is **closed**. The program persists its current internal state to disk upon exit, which will overwrite any changes made while the app is running.

**Location:** `~/.config/gomctools/config.toml`

### Config Breakdown

#### Global Settings
| Key | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `auto_load_previous_state` | bool | `true` | Automatically reloads the last used pack on startup. |

#### `[selector]`
| Key | Type | Description |
| :--- | :--- | :--- |
| `last_path` | string | The absolute path to the last loaded Minecraft instance. |

#### `[modlist]`
| Key | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `mode` | int | `0` | `0`: Merged, `1`: Split (Client/Server). |
| `output_format` | int | `0` | `0`: Markdown Bullets, `1`: GFM Table, `2`: BBCode. |
| `sort_field` | int | `0` | `0`: Name, `1`: Source, `2`: Side. |
| `sort_asc` | bool | `true` | `true`: Ascending, `false`: Descending. |
| `attach_links` | bool | `true` | Include Modrinth/CurseForge links. |
| `include_side` | bool | `true` | Include Client/Server tags. |
| `include_source` | bool | `true` | Include the mod's source platform. |
| `include_versions` | bool | `false` | Include compatible MC versions. |
| `include_filenames` | bool | `false` | Include actual `.jar` filenames. |
| `show_project_meta` | bool | `false` | Prepend pack name/author/version header. |
| `raw_preview` | bool | `false` | Show raw source instead of rendered preview. |

#### `[cleaner]`
The cleaner section allows you to define global custom presets.

**Example:**
```toml
[[cleaner.custom_presets]]
name = "BlueMap Web Folders"
pattern = "config/bluemap/web"
enabled = true
```

## Credits & Libraries

Built with the Charm stack:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - A powerful little TUI framework 🏗
- [Bubbles](https://charm.land/bubbles/v2) - TUI Components for Bubble Tea 🫧
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for nice terminal layouts 👄
- [Glamour](https://charm.land/glamour/v2) - Stylesheet-based markdown rendering for your CLI apps 💇🏻‍♀️
- [Bubblezone](https://github.com/lrstanley/bubblezone) - BubbleTea mouse event tracking utility

---



[![Cash App Badge](https://img.shields.io/badge/Cash%20App-00C244?logo=cashapp&logoColor=fff&style=flat)](https://cash.app/$ItzDavL)

*Created by ItzDabbzz* (First go project, kinda nevrous 👉👈)

Also made on ![Arch Linux Badge](https://img.shields.io/badge/Arch%20Linux-1793D1?logo=archlinux&logoColor=fff&style=flat)