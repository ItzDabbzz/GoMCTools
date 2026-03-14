package pages

// page_selector.go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/logger"
	"charm.land/lipgloss/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	pickerBorderInactive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)

	pickerBorderActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ui.HighlightColor).
				Padding(0, 1)

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

	fpFocused bool

	pageWidth  int
	pageHeight int
	contentW   int
	contentH   int
}

// ─── Constructor ──────────────────────────────────────────────────────────────

func NewSelectorPage(state *ui.SharedState) ui.Page {
	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.AllowedTypes = []string{}
	fp.ShowHidden = true

	fp.KeyMap.Open = key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "open dir"))
	fp.KeyMap.Back = key.NewBinding(key.WithKeys("h", "left", "esc", "backspace"), key.WithHelp("h/←/Esc/Bspace", "go up"))
	fp.KeyMap.Down = key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down"))
	fp.KeyMap.Up = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up"))

	if home, err := os.UserHomeDir(); err == nil {
		fp.CurrentDirectory = home
	}
	fp.AutoHeight = false
	fp.SetHeight(12)

	ti := textinput.New()
	ti.Placeholder = "Path to modpack"
	ti.CharLimit = 512
	ti.SetWidth(64)
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
		state:        state,
		fp:           fp,
		input:        ti,
		spin:         sp,
		spinning:     false,
		fpFocused:    false,
		selectedPath: lastPath,
		status:       "Type a path and press Enter — or press F to browse",
	}
}

// ─── ui.Page interface ────────────────────────────────────────────────────────

func (s *selectorPage) Title() string { return "Selector" }

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

	// Handle Enter key EARLY before other components consume it.
	// NOTE: bubbletea v2 KeyMsg.String() returns "enter", not "\n".
	_, isKeyMsg := msg.(tea.KeyMsg)
	if isKeyMsg {
		typed := msg.(tea.KeyMsg)
		if typed.String() == "enter" {
			// Browser mode: load the currently displayed directory.
			if s.fpFocused && s.fp.CurrentDirectory != "" {
				abs, err := filepath.Abs(s.fp.CurrentDirectory)
				if err == nil {
					s.selectedPath = abs
					s.input.SetValue(abs)
					s.status = fmt.Sprintf("Loading pack from %s…", filepath.Base(abs))
					s.spinning = true
					s.state.Config.Selector.LastPath = abs
					cmds = append(cmds, func() tea.Msg { return s.spin.Tick() }, ui.LoadPackCmd(abs))
					return s, tea.Batch(cmds...)
				}
			}
			// Text-input mode: validate and load from the typed path.
			if !s.fpFocused {
				trimmed := strings.TrimSpace(s.input.Value())
				if trimmed == "" {
					s.status = "Path cannot be empty"
					return s, tea.Batch(cmds...)
				}
				info, err := os.Stat(trimmed)
				if err != nil {
					logger.Error("Selector: path %q not found: %v", trimmed, err)
					s.status = "Path not found"
					return s, tea.Batch(cmds...)
				}
				if !info.IsDir() {
					logger.Error("Selector: path %q is not a directory", trimmed)
					s.status = "Path is not a directory"
					return s, tea.Batch(cmds...)
				}
				abs, err := filepath.Abs(trimmed)
				if err != nil {
					logger.Error("Selector: could not resolve absolute path for %q: %v", trimmed, err)
					s.status = "Could not resolve absolute path"
					return s, tea.Batch(cmds...)
				}
				s.selectedPath = abs
				s.status = fmt.Sprintf("Loading pack from %s…", filepath.Base(abs))
				s.fp.CurrentDirectory = abs
				s.input.SetValue("")
				s.spinning = true
				s.state.Config.Selector.LastPath = abs
				cmds = append(cmds, s.fp.Init(), func() tea.Msg { return s.spin.Tick() }, ui.LoadPackCmd(abs))
				return s, tea.Batch(cmds...)
			}
		}
	}

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

	// Handle q to exit browser.
	if isKeyMsg && s.fpFocused {
		typed := msg.(tea.KeyMsg)
		if typed.String() == "q" {
			s.fpFocused = false
			s.input.Focus()
			s.status = "Type a path and press Enter — or press F to browse"
			return s, nil
		}
	}

	// Forward messages to filepicker.
	if !isKeyMsg || s.fpFocused {
		prevDir := s.fp.CurrentDirectory
		var fpCmd tea.Cmd
		s.fp, fpCmd = s.fp.Update(msg)
		if fpCmd != nil {
			cmds = append(cmds, fpCmd)
		}
		// Sync text input and status when the browser navigates to a new directory.
		if s.fpFocused && s.fp.CurrentDirectory != prevDir && s.fp.CurrentDirectory != "" {
			s.input.SetValue(s.fp.CurrentDirectory)
			s.status = fmt.Sprintf("Browsing: %s", s.fp.CurrentDirectory)
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
		// Clear status back to a neutral ready-state after a successful load.
		s.status = fmt.Sprintf("✓ Loaded %d mods from %s",
			typed.Info.Counts.Total, filepath.Base(typed.Info.InstancePath))
		s.state.Config.Selector.LastPath = s.selectedPath
		return s, tea.Batch(cmds...)

	case ui.ContentSizeMsg:
		s.contentW = typed.Width
		s.contentH = typed.Height
		s.updateLayout()

	case tea.WindowSizeMsg:
		s.pageWidth = typed.Width
		s.pageHeight = typed.Height

	case tea.KeyMsg:
		if key.Matches(typed, ui.DefaultKeys.Quit) {
			return s, tea.Quit
		}
		if key.Matches(typed, ui.DefaultKeys.Help) {
			return s, func() tea.Msg { return ui.ToggleHelpMsg{} }
		}

		if !s.fpFocused && (typed.String() == "f" || typed.String() == "\t") {
			s.fpFocused = true
			s.input.Blur()
			s.status = "Browsing — l/→ to open, Enter to load, q to exit"
			if trimmed := strings.TrimSpace(s.input.Value()); trimmed != "" {
				if info, err := os.Stat(trimmed); err == nil && info.IsDir() {
					s.fp.CurrentDirectory = trimmed
					cmds = append(cmds, s.fp.Init())
				}
			}
			return s, tea.Batch(cmds...)
		}
	}

	return s, tea.Batch(cmds...)
}

// ─── Layout ───────────────────────────────────────────────────────────────────

func (s *selectorPage) updateLayout() {
	if s.contentW == 0 || s.contentH == 0 {
		return
	}

	innerWidth := s.contentW - pickerBorderInactive.GetHorizontalFrameSize() - 2
	if innerWidth < 48 {
		innerWidth = 48
	}
	s.input.SetWidth(innerWidth)

	reserved := pickerBorderInactive.GetVerticalFrameSize() + 10
	usableHeight := s.contentH - reserved
	if usableHeight < 6 {
		usableHeight = 6
	}
	s.fp.SetHeight(usableHeight)
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (s *selectorPage) View() string {
	var b strings.Builder

	centeredStyle := lipgloss.NewStyle().
		Width(s.contentW).
		Align(lipgloss.Center)

	// ── Selected directory ────────────────────────────────────────────────
	b.WriteString(labelStyle.Render("Directory: "))
	b.WriteString(valueStyle.Render(s.selectedPath))
	b.WriteString("\n")

	// ── Status / spinner ──────────────────────────────────────────────────
	// Always render s.status — the spinner is a prefix decoration only.
	if s.spinning {
		b.WriteString(s.spin.View())
		b.WriteString(" ")
	}
	if s.status != "" {
		b.WriteString(statusStyle.Render(s.status))
	}
	b.WriteString("\n")

	// ── Text input ────────────────────────────────────────────────────────
	b.WriteString(labelStyle.Render("Path:"))
	b.WriteString("\n")
	b.WriteString(s.input.View())
	b.WriteString("\n")

	// ── Filepicker with focus-aware border ────────────────────────────────
	var hint string
	borderStyle := pickerBorderInactive
	if s.fpFocused {
		borderStyle = pickerBorderActive
		hint = pickerHintStyle.Render("↑/↓ or j/k navigate  ←/h/esc go up  l/→ open  Enter load  q exit")
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

	return centeredStyle.Render(b.String())
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
