# GoMCTools

## Description

A TUI based tool for helping minecraft modpack developers release modpacks easier.

## Todo

- In pack cleaner, setup a way to clean out the maps folder in `/config/bluemap/maps/` to prevent errors

## Credits/Libraries

- [Go](https://go.dev)
- [Bubbles](https://github.com/charmbracelet/bubbles.git)
- [Glow](https://github.com/charmbracelet/glow.git)
- [Lipgloss](https://github.com/charmbracelet/lipgloss.git)
- [Bubblezone](https://github.com/lrstanley/bubblezone.git)
- [BubbleTea](https://github.com/charmbracelet/bubbletea.git)

## Recent UI improvements

- Added a spinner to the Selector page while a pack is loading to improve feedback.
- Unified per-page short-help rendering in the footer so each tab can show relevant key hints.
- Added `?` help support for modlist (full help overlay) and standardized key bindings for modlist/cleaner pages.
- Plan: add tests for pack parsing and cleaner logic next.
