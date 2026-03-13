package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"itzdabbzz.me/gomctools/internal/ui"
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	// pickerBorderInactive is shown while the filepicker is not focused.
	pickerBorderInactive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)

	// pickerBorderActive is shown while the filepicker owns arrow-key input.
	pickerBorderActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ui.HighlightColor).
				Padding(0, 1)

	// pickerHint nudges the user to activate the browser.
	pickerHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

// ─── Page model ───────────────────────────────────────────────────────────────

type selectorPage struct {
	selectedPath string
	status       string
	state        *ui.SharedState

	fp       filepicker.Model
	input    textinput.Model
	spin     spinner.Model
	spinning bool

	// fpFocused controls whether the filepicker receives arrow-key input and
	// whether CaptureGlobalNav returns true.  False by default so that the
	// root tab-bar can still use arrow keys when the user first arrives on
	// this page.
	fpFocused bool

	pageWidth  int
	pageHeight int
}

// ─── Constructor ──────────────────────────────────────────────────────────────

func NewSelectorPage(state *ui.SharedState) ui.Page {
	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.AllowedTypes = []string{}
	fp.ShowHidden = true

	fp.KeyMap.Open = key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "open dir"))
	fp.KeyMap.Select = filepicker.DefaultKeyMap().Select

	if home, err := os.UserHomeDir(); err == nil {
		fp.CurrentDirectory = home
	}
	fp.AutoHeight = false
	fp.Height = 12

	ti := textinput.New()
	ti.Placeholder = "Path to modpack"
	ti.CharLimit = 512
	ti.Width = 64
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.HighlightColor)

	lastPath := "No directory selected"
	if state.Config.AutoLoadPreviousState && state.Config.Selector.LastPath != "" {
		lastPath = state.Config.Selector.LastPath
		ti.SetValue(lastPath)
		fp.CurrentDirectory = lastPath
	}

	return &selectorPage{
		selectedPath: lastPath,
		status:       "Type a path and press Enter — or press F to browse",
		state:        state,
		fp:           fp,
		input:        ti,
		spin:         sp,
	}
}

// ─── ui.Page interface ────────────────────────────────────────────────────────

func (s *selectorPage) Title() string { return "Selector" }

// CaptureGlobalNav only returns true while the filepicker is focused.
// When false the root tab-bar receives arrow keys as normal.
func (s *selectorPage) CaptureGlobalNav() bool { return s.fpFocused }

func (s *selectorPage) Init() tea.Cmd {
	cmds := []tea.Cmd{s.fp.Init(), textinput.Blink}

	if s.state.Config.AutoLoadPreviousState && s.selectedPath != "No directory selected" {
		abs, err := filepath.Abs(s.selectedPath)
		if err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				cmds = append(cmds, ui.LoadPackCmd(abs))
				s.status = "Auto-loading last pack…"
				s.spinning = true
			}
		}
	}

	return tea.Batch(cmds...)
}

func (s *selectorPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	var cmds []tea.Cmd

	// Always route messages to the text input.
	var inputCmd tea.Cmd
	s.input, inputCmd = s.input.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}

	// Keep the spinner ticking while loading.
	if s.spinning {
		var spinCmd tea.Cmd
		s.spin, spinCmd = s.spin.Update(msg)
		if spinCmd != nil {
			cmds = append(cmds, spinCmd)
		}
	}

	// Always forward non-keyboard messages to the filepicker so its internal
	// directory-scan goroutines can populate the listing regardless of focus.
	// Keyboard input is only forwarded when the picker is explicitly focused,
	// which prevents arrow keys from leaking into the picker while navigating tabs.
	_, isKeyMsg := msg.(tea.KeyMsg)
	if !isKeyMsg || s.fpFocused {
		var fpCmd tea.Cmd
		s.fp, fpCmd = s.fp.Update(msg)
		if fpCmd != nil {
			cmds = append(cmds, fpCmd)
		}
	}

	switch typed := msg.(type) {

	case ui.PackLoadedMsg:
		s.spinning = false
		if typed.Err != nil {
			s.status = fmt.Sprintf("Failed to load pack: %v", typed.Err)
			return s, tea.Batch(cmds...)
		}
		s.selectedPath = typed.Info.InstancePath
		s.status = fmt.Sprintf("✓  Loaded %d mods from %s",
			typed.Info.Counts.Total, filepath.Base(typed.Info.InstancePath))
		s.state.Config.Selector.LastPath = s.selectedPath
		return s, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		s.pageWidth = typed.Width
		s.pageHeight = typed.Height
		s.updateLayout()

	case tea.KeyMsg:
		// Quit / help always work regardless of focus state.
		if key.Matches(typed, ui.DefaultKeys.Quit) {
			return s, tea.Quit
		}
		if key.Matches(typed, ui.DefaultKeys.Help) {
			return s, func() tea.Msg { return ui.ToggleHelpMsg{} }
		}

		// ── Filepicker focus management ────────────────────────────────────
		// Escape releases the filepicker and gives arrow keys back to the
		// root tab-bar.
		if s.fpFocused && typed.Type == tea.KeyEsc {
			s.fpFocused = false
			s.input.Focus()
			s.status = "Type a path and press Enter — or press F to browse"
			return s, tea.Batch(cmds...)
		}

		// F (or Tab) activates the filepicker browser when it is not focused.
		if !s.fpFocused && (typed.String() == "f" || typed.Type == tea.KeyTab) {
			s.fpFocused = true
			s.input.Blur()
			s.status = "Browsing — Enter to load, Esc to exit browser"
			// Sync the picker to whatever the text input contains, if valid.
			if trimmed := strings.TrimSpace(s.input.Value()); trimmed != "" {
				if info, err := os.Stat(trimmed); err == nil && info.IsDir() {
					s.fp.CurrentDirectory = trimmed
					cmds = append(cmds, s.fp.Init())
				}
			}
			return s, tea.Batch(cmds...)
		}

		// ── Text-input Enter → load pack ──────────────────────────────────
		// Only process Enter when the filepicker is not capturing input.
		if !s.fpFocused && typed.Type == tea.KeyEnter {
			trimmed := strings.TrimSpace(s.input.Value())
			if trimmed == "" {
				s.status = "Path cannot be empty"
				return s, tea.Batch(cmds...)
			}
			info, err := os.Stat(trimmed)
			if err != nil {
				s.status = "Path not found"
				return s, tea.Batch(cmds...)
			}
			if !info.IsDir() {
				s.status = "Path is not a directory"
				return s, tea.Batch(cmds...)
			}
			abs, err := filepath.Abs(trimmed)
			if err != nil {
				s.status = "Could not resolve absolute path"
				return s, tea.Batch(cmds...)
			}
			s.selectedPath = abs
			s.status = "Loading pack…"
			s.fp.CurrentDirectory = abs
			s.input.SetValue("")
			s.spinning = true
			s.state.Config.Selector.LastPath = abs
			cmds = append(cmds, s.fp.Init(), spinner.Tick, ui.LoadPackCmd(abs))
			return s, tea.Batch(cmds...)
		}

		// ── Filepicker Enter → load selected directory ────────────────────
		if s.fpFocused && typed.Type == tea.KeyEnter {
			dir := s.fp.CurrentDirectory
			if dir == "" {
				return s, tea.Batch(cmds...)
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				s.status = "Could not resolve path"
				return s, tea.Batch(cmds...)
			}
			s.fpFocused = false
			s.input.Focus()
			s.selectedPath = abs
			s.status = "Loading pack…"
			s.input.SetValue(abs)
			s.spinning = true
			s.state.Config.Selector.LastPath = abs
			cmds = append(cmds, spinner.Tick, ui.LoadPackCmd(abs))
			return s, tea.Batch(cmds...)
		}
	}

	return s, tea.Batch(cmds...)
}

// ─── Layout ───────────────────────────────────────────────────────────────────

func (s *selectorPage) updateLayout() {
	if s.pageWidth == 0 || s.pageHeight == 0 {
		return
	}
	// Account for doc + window frames plus the picker border (2 sides × 1 col padding + 1 border).
	hFrame := ui.DocStyle.GetHorizontalFrameSize() +
		ui.WindowStyle.GetHorizontalFrameSize() +
		pickerBorderInactive.GetHorizontalFrameSize()
	innerWidth := s.pageWidth - hFrame - 2
	if innerWidth < 48 {
		innerWidth = 48
	}
	s.input.Width = innerWidth

	// Reserve rows for: dir line, status, blank, path label, input, blank,
	// hint, and the border's vertical frame.
	reserved := ui.DocStyle.GetVerticalFrameSize() +
		ui.WindowStyle.GetVerticalFrameSize() +
		pickerBorderInactive.GetVerticalFrameSize() +
		7 // dir + status + gap + label + input + gap + hint
	usableHeight := s.pageHeight - reserved
	if usableHeight < 6 {
		usableHeight = 6
	}
	s.fp.Height = usableHeight
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (s *selectorPage) View() string {
	var b strings.Builder

	// ── Selected directory ────────────────────────────────────────────────
	b.WriteString(labelStyle.Render("Directory: "))
	b.WriteString(valueStyle.Render(s.selectedPath))
	b.WriteString("\n")

	// ── Status / spinner ──────────────────────────────────────────────────
	if s.spinning {
		b.WriteString(s.spin.View())
		b.WriteString(statusStyle.Render(" Loading pack…"))
	} else if s.status != "" {
		b.WriteString(statusStyle.Render(s.status))
	}
	b.WriteString("\n\n")

	// ── Text input ────────────────────────────────────────────────────────
	b.WriteString(labelStyle.Render("Path:"))
	b.WriteString("\n")
	b.WriteString(s.input.View())
	b.WriteString("\n\n")

	// ── Filepicker with focus-aware border ────────────────────────────────
	var hint string
	borderStyle := pickerBorderInactive
	if s.fpFocused {
		borderStyle = pickerBorderActive
		hint = pickerHintStyle.Render("↑/↓ navigate  l/→ open  Enter load  Esc exit browser")
	} else {
		hint = pickerHintStyle.Render("Press F (or Tab) to activate the directory browser")
	}

	pickerTitle := labelStyle.Render("Browse:")
	pickerBox := borderStyle.Render(s.fp.View())
	b.WriteString(pickerTitle)
	b.WriteString("\n")
	b.WriteString(pickerBox)
	b.WriteString("\n")
	b.WriteString(hint)

	return b.String()
}

// ─── Key help ─────────────────────────────────────────────────────────────────

type selectorKeyMap struct {
	LoadPack   key.Binding
	ActivateFP key.Binding
	ExitFP     key.Binding
	OpenDir    key.Binding
}

func (k selectorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.LoadPack, k.ActivateFP, k.ExitFP, k.OpenDir}
}

func (k selectorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

var selectorKeys = selectorKeyMap{
	LoadPack: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "load pack"),
	),
	ActivateFP: key.NewBinding(
		key.WithKeys("f", "tab"),
		key.WithHelp("f/tab", "activate browser"),
	),
	ExitFP: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "exit browser"),
	),
	OpenDir: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/→", "open dir"),
	),
}

func (s *selectorPage) ShortHelp() []key.Binding  { return selectorKeys.ShortHelp() }
func (s *selectorPage) FullHelp() [][]key.Binding { return selectorKeys.FullHelp() }
