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

type selectorPage struct {
	selectedPath string
	status       string
	state        *ui.SharedState
	fp           filepicker.Model
	input        textinput.Model
	spin         spinner.Model
	spinning     bool
	pageWidth    int
	pageHeight   int
}

func NewSelectorPage(state *ui.SharedState) ui.Page {
	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.AllowedTypes = []string{}
	fp.ShowHidden = true

	// Keep enter for selection, remove it from navigation so selecting a dir
	// does not also descend into it.
	defaultKeys := filepicker.DefaultKeyMap()
	fp.KeyMap.Open = key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/right", "open"))
	fp.KeyMap.Select = defaultKeys.Select
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

	// Load last path from config if auto-load is enabled
	lastPath := "No directory selected"
	if state.Config.AutoLoadPreviousState && state.Config.Selector.LastPath != "" {
		lastPath = state.Config.Selector.LastPath
		ti.SetValue(lastPath)
		fp.CurrentDirectory = lastPath
	}

	return &selectorPage{
		selectedPath: lastPath,
		status:       "Paste or type a path, press Enter to load (showing hidden)",
		state:        state,
		fp:           fp,
		input:        ti,
		spin:         sp,
		spinning:     false,
	}
}

func (s *selectorPage) Title() string {
	return "Selector"
}

func (s *selectorPage) Init() tea.Cmd {
	cmds := []tea.Cmd{s.fp.Init(), textinput.Blink}

	// Auto-load last pack if enabled and path exists
	if s.state.Config.AutoLoadPreviousState && s.selectedPath != "No directory selected" {
		abs, err := filepath.Abs(s.selectedPath)
		if err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				cmds = append(cmds, ui.LoadPackCmd(abs))
				s.status = "Auto-loading last pack..."
				s.spinning = true
			}
		}
	}

	return tea.Batch(cmds...)
}

func (s *selectorPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	var cmds []tea.Cmd
	var inputCmd tea.Cmd
	s.input, inputCmd = s.input.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}

	if s.spinning {
		var spinCmd tea.Cmd
		s.spin, spinCmd = s.spin.Update(msg)
		if spinCmd != nil {
			cmds = append(cmds, spinCmd)
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
		s.status = fmt.Sprintf("Loaded %d mods from %s", typed.Info.Counts.Total, filepath.Base(typed.Info.InstancePath))
		// Save the path to config
		s.state.Config.Selector.LastPath = s.selectedPath
		s.state.Config.LastPackPath = s.selectedPath
		return s, tea.Batch(cmds...)
	case tea.WindowSizeMsg:
		s.pageWidth = typed.Width
		s.pageHeight = typed.Height
		s.updateLayout()
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEnter:
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
			s.status = "Loading pack..."
			s.fp.CurrentDirectory = abs
			s.input.SetValue("")
			s.spinning = true

			// Save the path to config immediately (even if pack load fails)
			s.state.Config.Selector.LastPath = abs
			s.state.Config.LastPackPath = abs

			cmds = append(cmds, s.fp.Init(), spinner.Tick, ui.LoadPackCmd(abs))
		}

		return s, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	s.fp, cmd = s.fp.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return s, tea.Batch(cmds...)
}

func (s *selectorPage) updateLayout() {
	if s.pageWidth == 0 || s.pageHeight == 0 {
		return
	}
	innerWidth := s.pageWidth - ui.DocStyle.GetHorizontalFrameSize() - ui.WindowStyle.GetHorizontalFrameSize() - 2
	if innerWidth < 48 {
		innerWidth = 48
	}
	s.input.Width = innerWidth

	usableHeight := s.pageHeight - ui.DocStyle.GetVerticalFrameSize() - ui.WindowStyle.GetVerticalFrameSize() - 6
	if usableHeight < 8 {
		usableHeight = 8
	}

	s.fp.Height = usableHeight
}

func (s *selectorPage) View() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Modpack directory: %s\n", s.selectedPath))

	if s.spinning {
		builder.WriteString(fmt.Sprintf("%s Loading pack...\n", s.spin.View()))
	} else if s.status != "" {
		builder.WriteString(s.status)
		builder.WriteString("\n")
	}

	builder.WriteString("Path:\n")
	builder.WriteString(s.input.View())
	builder.WriteString("\n\n")

	builder.WriteString(s.fp.View())
	return builder.String()
}

func (s *selectorPage) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "load pack")),
		key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/right", "open")),
	}
}

func (s *selectorPage) FullHelp() [][]key.Binding { return [][]key.Binding{s.ShortHelp()} }
