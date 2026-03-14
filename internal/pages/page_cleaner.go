package pages

//page.cleaner.go
import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/config"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
	zone "github.com/lrstanley/bubblezone/v2"
)

// cleanerInputMode describes whether the user is currently editing a preset.
type cleanerInputMode int

const (
	cleanerInputNone cleanerInputMode = iota
	cleanerInputAdd
	cleanerInputEdit
)

// cleanerPage is the Pack Cleaner page that lets users selectively delete
// generated or unwanted files from a loaded Prism Launcher instance.
type cleanerPage struct {
	state     *ui.SharedState
	zone      *zone.Manager
	prefix    string
	presets   []cleanerPreset
	keys      cleanerKeyMap
	helpWidth int

	selected int

	inputMode cleanerInputMode
	inputName textinput.Model
	inputPath textinput.Model
	editIndex int
	focusIdx  int

	status       string
	lastDuration time.Duration
	elapsed      time.Duration
	totalItems   int
	processed    int
	list         viewport.Model

	running    bool
	progress   progress.Model
	workQueue  []cleanerPreset
	workStart  time.Time
	workErrors []string

	pageWidth    int
	pageHeight   int
	contentWidth int // set from ContentSizeMsg; exact inner width for layout

	saving    bool
	debugMode bool
}

// NewCleanerPage constructs a new Pack Cleaner page backed by state.
func NewCleanerPage(state *ui.SharedState) ui.Page {
	nameInput := textinput.New()
	nameInput.Placeholder = "Preset name"
	nameInput.CharLimit = 64
	nameInput.Prompt = ""

	pathInput := textinput.New()
	pathInput.Placeholder = "Path or folder"
	pathInput.CharLimit = 256
	pathInput.Prompt = ""

	pr := progress.New(progress.WithColors(ui.HighlightColor.Light))
	pr.SetWidth(32)

	list := viewport.New(viewport.WithWidth(cleanerColWidth), viewport.WithHeight(cleanerColHeight))
	list.MouseWheelEnabled = true
	list.MouseWheelDelta = 2

	// Merge built-in presets with any previously saved custom ones.
	presets := defaultCleanerPresets()
	if len(state.Config.Cleaner.CustomPresets) > 0 {
		custom := make([]cleanerPreset, 0, len(state.Config.Cleaner.CustomPresets))
		for _, cp := range state.Config.Cleaner.CustomPresets {
			custom = append(custom, cleanerPreset{
				Name:    cp.Name,
				Pattern: cp.Pattern,
				Enabled: cp.Enabled,
				BuiltIn: false,
			})
		}
		presets = append(presets, normalizeCustom(custom)...)
	}

	return &cleanerPage{
		state:      state,
		keys:       defaultCleanerKeyMap(),
		presets:    presets,
		inputName:  nameInput,
		inputPath:  pathInput,
		progress:   pr,
		list:       list,
		status:     "",
		editIndex:  -1,
		focusIdx:   0,
		workErrors: nil,
	}
}

// SetZone wires the page to the root bubblezone manager.
func (c *cleanerPage) SetZone(z *zone.Manager, prefix string) {
	c.zone = z
	c.prefix = prefix
}

func (c *cleanerPage) Title() string { return "Pack Cleaner" }
func (c *cleanerPage) Init() tea.Cmd { return nil }

// CaptureGlobalNav returns true while a text input is active, preventing
// tab/shift-tab from switching pages unexpectedly.
func (c *cleanerPage) CaptureGlobalNav() bool {
	return c.inputMode != cleanerInputNone
}

func (c *cleanerPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch m := msg.(type) {
	case ui.PackLoadedMsg:
		if m.Err != nil {
			c.status = fmt.Sprintf("Load failed: %v", m.Err)
			return c, nil
		}
		c.status = fmt.Sprintf("Loaded %s. Select items to clean.", filepath.Base(m.Info.InstancePath))
		return c, c.loadCustomPresetsCmd(m.Info.MinecraftDir)
	case tea.KeyMsg:
		return c.handleKey(m)
	case cleanerTickMsg:
		if c.running {
			c.elapsed = time.Since(c.workStart)
			return c, c.tickElapsed()
		}
	case cleanerStepMsg:
		if m.Err != nil {
			c.workErrors = append(c.workErrors, fmt.Sprintf("%s: %v", m.Preset, m.Err))
		}
		c.processed += m.Count
		percent := c.progressPercent(m.Index, m.Total)
		c.progress.SetPercent(percent)
		c.status = fmt.Sprintf("Cleaning %s (%d/%d)", m.Preset, m.Index+1, m.Total)
		if m.Index+1 < m.Total {
			return c, c.runCleanStepCmd(m.Index + 1)
		}
		return c, func() tea.Msg {
			return cleanerDoneMsg{Duration: time.Since(c.workStart), Errors: c.workErrors}
		}
	case cleanerDoneMsg:
		c.running = false
		c.elapsed = m.Duration
		c.lastDuration = m.Duration
		if len(m.Errors) > 0 {
			c.status = fmt.Sprintf("Finished with %d error(s).", len(m.Errors))
		} else {
			c.status = "Clean completed successfully."
		}
		c.workQueue = nil
		c.workErrors = nil
		c.progress.SetPercent(1)
	case cleanerLoadedMsg:
		if m.Err != nil {
			c.status = fmt.Sprintf("Config load failed: %v", m.Err)
			return c, nil
		}
		c.mergePresets(m.Custom)
		c.status = "Presets loaded. Toggle and press c to clean."
	case cleanerSavedMsg:
		if m.Err != nil {
			c.status = fmt.Sprintf("Save failed: %v", m.Err)
		} else {
			c.status = "Presets saved."
		}
		c.saving = false
	case tea.WindowSizeMsg:
		// Store raw terminal size for availableContentWidth() width calculation.
		c.pageWidth = m.Width
		c.pageHeight = m.Height
	case ui.ContentSizeMsg:
		// Use the exact content area dimensions the model has already computed
		// rather than guessing at frame overhead ourselves.
		c.contentWidth = m.Width
		const minHorizontalWidth = 82
		isHorizontal := m.Width >= minHorizontalWidth

		// Reserve one line for the status bar rendered below the columns.
		listHeight := m.Height - 1
		if listHeight < cleanerColHeight {
			listHeight = cleanerColHeight
		}

		if isHorizontal {
			c.list.SetWidth(cleanerListWidth())
			c.list.SetHeight(listHeight)
			rightWidth := m.Width - cleanerColWidth - cleanerGapWidth
			if rightWidth < 32 {
				rightWidth = 32
			}
			c.setHelpWidth(rightWidth)
		} else {
			// Stacked: split height evenly between list and detail panel.
			stackedHeight := listHeight / 2
			if stackedHeight < 8 {
				stackedHeight = 8
			}
			c.list.SetWidth(m.Width)
			c.list.SetHeight(stackedHeight)
			c.setHelpWidth(m.Width)
		}
		c.ensureSelectionVisible()
	}

	var cmds []tea.Cmd
	if c.inputMode != cleanerInputNone {
		var cmd tea.Cmd
		c.inputName, cmd = c.inputName.Update(msg)
		cmds = append(cmds, cmd)
		c.inputPath, cmd = c.inputPath.Update(msg)
		cmds = append(cmds, cmd)
	}

	if c.running && len(cmds) == 0 {
		cmds = append(cmds, c.tickElapsed())
	}

	var vpCmd tea.Cmd
	c.list, vpCmd = c.list.Update(msg)
	cmds = append(cmds, vpCmd)

	return c, tea.Batch(cmds...)
}

func (c *cleanerPage) View() string {
	contentWidth := c.contentWidth
	if contentWidth < 40 {
		contentWidth = 80 // Default reasonable width
	}
	const minHorizontalWidth = 82
	isHorizontal := contentWidth >= minHorizontalWidth

	var body string
	if isHorizontal {
		leftTotal := cleanerColWidth
		rightTotal := contentWidth - leftTotal - cleanerGapWidth
		if rightTotal < 32 {
			rightTotal = 32
		}
		left := c.viewPresetList(leftTotal)
		right := c.viewDetail(rightTotal)
		spacer := strings.Repeat(" ", cleanerGapWidth)
		row := lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
		body = lipgloss.NewStyle().Width(contentWidth).Render(row)
	} else {
		left := c.viewPresetList(contentWidth)
		right := c.viewDetail(contentWidth)
		vertical := lipgloss.JoinVertical(lipgloss.Left, left, right)
		body = lipgloss.NewStyle().Width(contentWidth).Render(vertical)
	}

	if c.status != "" {
		body += "\n" + statusStyle.Render(c.status)
	}

	if c.debugMode {
		debugInfo := fmt.Sprintf("DEBUG: %dx%d | W:%d H:%d", c.pageWidth, c.pageHeight, contentWidth, c.list.Height())
		debugLine := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(debugInfo)
		body = debugLine + "\n" + body
	}

	return body
}

// --- key handling ---

func (c *cleanerPage) handleKey(msg tea.KeyMsg) (ui.Page, tea.Cmd) {
	if c.inputMode != cleanerInputNone {
		switch msg.String() {
		case "esc":
			c.exitInput()
			return c, nil
		case "tab", "shift+tab":
			c.focusIdx = 1 - c.focusIdx
			c.updateFocus()
			return c, nil
		case "enter":
			if c.saveInput() {
				return c, c.saveCustomPresetsCmd()
			}
			return c, nil
		}
		return c, nil
	}

	switch {
	case key.Matches(msg, c.keys.Up):
		if c.selected > 0 {
			c.selected--
		}
		c.ensureSelectionVisible()
	case key.Matches(msg, c.keys.Down):
		if c.selected < len(c.presets)-1 {
			c.selected++
		}
		c.ensureSelectionVisible()
	case key.Matches(msg, c.keys.Toggle):
		if c.selected >= 0 && c.selected < len(c.presets) {
			c.presets[c.selected].Enabled = !c.presets[c.selected].Enabled
		}
	case key.Matches(msg, c.keys.Clean):
		return c.startClean()
	case key.Matches(msg, c.keys.New):
		c.enterInput(cleanerInputAdd)
	case key.Matches(msg, c.keys.Edit):
		if c.selected >= 0 && c.selected < len(c.presets) && !c.presets[c.selected].BuiltIn {
			c.enterInput(cleanerInputEdit)
		}
	case key.Matches(msg, c.keys.Delete):
		if c.selected >= 0 && c.selected < len(c.presets) && !c.presets[c.selected].BuiltIn {
			c.deleteSelected()
			return c, c.saveCustomPresetsCmd()
		}
	case key.Matches(msg, c.keys.Save):
		return c, c.saveCustomPresetsCmd()
	case key.Matches(msg, c.keys.Debug):
		c.debugMode = !c.debugMode
		return c, nil
	}

	return c, nil
}

func (c *cleanerPage) enterInput(mode cleanerInputMode) {
	c.inputMode = mode
	c.focusIdx = 0
	c.inputName.SetValue("")
	c.inputPath.SetValue("")
	c.updateFocus()

	if mode == cleanerInputEdit && c.selected >= 0 && c.selected < len(c.presets) {
		p := c.presets[c.selected]
		c.inputName.SetValue(p.Name)
		c.inputPath.SetValue(p.Pattern)
		c.editIndex = c.selected
	} else {
		c.editIndex = -1
	}
}

func (c *cleanerPage) exitInput() {
	c.inputMode = cleanerInputNone
	c.editIndex = -1
	c.inputName.Blur()
	c.inputPath.Blur()
}

func (c *cleanerPage) updateFocus() {
	if c.focusIdx == 0 {
		c.inputName.Focus()
		c.inputPath.Blur()
	} else {
		c.inputPath.Focus()
		c.inputName.Blur()
	}
}

func (c *cleanerPage) saveInput() bool {
	name := strings.TrimSpace(c.inputName.Value())
	pattern := strings.TrimSpace(c.inputPath.Value())
	if name == "" || pattern == "" {
		c.status = "Name and path are required."
		return false
	}

	preset := cleanerPreset{Name: name, Pattern: normalizePattern(pattern), Enabled: true, BuiltIn: false}
	if c.inputMode == cleanerInputAdd {
		c.presets = append(c.presets, preset)
		c.selected = len(c.presets) - 1
	} else if c.inputMode == cleanerInputEdit && c.editIndex >= 0 && c.editIndex < len(c.presets) {
		preset.Enabled = c.presets[c.editIndex].Enabled
		c.presets[c.editIndex] = preset
	}

	c.exitInput()
	c.status = "Preset saved locally (press s to persist)."
	return true
}

func (c *cleanerPage) deleteSelected() {
	if c.selected < 0 || c.selected >= len(c.presets) {
		return
	}
	c.presets = append(c.presets[:c.selected], c.presets[c.selected+1:]...)
	if c.selected >= len(c.presets) {
		c.selected = len(c.presets) - 1
	}
	c.status = "Preset removed."
}

func (c *cleanerPage) mergePresets(custom []cleanerPreset) {
	builtIns := defaultCleanerPresets()
	c.presets = append(builtIns, normalizeCustom(custom)...)
	sort.SliceStable(c.presets, func(i, j int) bool { return c.presets[i].Name < c.presets[j].Name })
	c.selected = ui.ClampInt(c.selected, 0, len(c.presets)-1)
}

// --- cleaning commands ---

func (c *cleanerPage) startClean() (ui.Page, tea.Cmd) {
	if c.running {
		return c, nil
	}
	if c.state == nil || c.state.Pack.MinecraftDir == "" {
		c.status = "Load a pack first."
		return c, nil
	}

	enabled := make([]cleanerPreset, 0, len(c.presets))
	for _, p := range c.presets {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	if len(enabled) == 0 {
		c.status = "Select at least one preset to clean."
		return c, nil
	}

	total := 0
	for _, p := range enabled {
		count, _ := countEntries(c.state.Pack.MinecraftDir, p)
		total += count
	}
	if total == 0 {
		total = len(enabled)
	}

	c.workQueue = enabled
	c.workStart = time.Now()
	c.elapsed = 0
	c.running = true
	c.progress.SetPercent(0)
	c.workErrors = nil
	c.status = "Starting cleaner..."
	c.totalItems = total
	c.processed = 0

	return c, tea.Batch(c.runCleanStepCmd(0), c.tickElapsed())
}

func (c *cleanerPage) tickElapsed() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg { return cleanerTickMsg{} })
}

func (c *cleanerPage) runCleanStepCmd(index int) tea.Cmd {
	if index < 0 || index >= len(c.workQueue) {
		return nil
	}
	preset := c.workQueue[index]
	root := c.state.Pack.MinecraftDir
	return func() tea.Msg {
		count, err := deletePreset(root, preset)
		return cleanerStepMsg{Index: index, Total: len(c.workQueue), Preset: preset.Name, Err: err, Count: count}
	}
}

func (c *cleanerPage) loadCustomPresetsCmd(root string) tea.Cmd {
	return func() tea.Msg {
		custom, err := readCleanerConfig(root)
		return cleanerLoadedMsg{Custom: custom, Err: err}
	}
}

func (c *cleanerPage) saveCustomPresetsCmd() tea.Cmd {
	if c.state == nil {
		c.status = "State not initialized."
		return nil
	}
	c.saving = true
	presets := filterCustom(c.presets)
	c.saveToGlobalConfig()

	if c.state.Pack.MinecraftDir != "" {
		root := c.state.Pack.MinecraftDir
		return func() tea.Msg {
			err := writeCleanerConfig(root, presets)
			return cleanerSavedMsg{Err: err}
		}
	}
	return func() tea.Msg { return cleanerSavedMsg{Err: nil} }
}

// saveToGlobalConfig copies the current custom presets into the shared config
// so they are persisted to disk when the application exits.
func (c *cleanerPage) saveToGlobalConfig() {
	if c.state == nil || c.state.Config == nil {
		return
	}
	presets := filterCustom(c.presets)
	c.state.Config.Cleaner.CustomPresets = make([]config.CleanerPreset, 0, len(presets))
	for _, p := range presets {
		c.state.Config.Cleaner.CustomPresets = append(c.state.Config.Cleaner.CustomPresets, config.CleanerPreset{
			Name:    p.Name,
			Pattern: p.Pattern,
			Enabled: p.Enabled,
		})
	}
}

// --- view helpers ---

func (c *cleanerPage) viewPresetList(totalWidth int) string {
	if len(c.presets) == 0 {
		c.list.SetContent("No presets")
		return cleanerLeftColStyle.Width(totalWidth).Render(c.list.View())
	}

	lines := make([]string, 0, len(c.presets)+1)
	lines = append(lines, sectionTitleStyle.Render("Presets"))
	for _, p := range c.presets {
		check := checkbox(p.Enabled)
		label := fmt.Sprintf("%s %s", check, p.Name)
		if p.BuiltIn {
			label += " • default"
		}
		lines = append(lines, label)
	}
	// Style each preset line
	for i := range lines {
		if i == 0 {
			continue // section title
		}
		if i-1 == c.selected {
			lines[i] = presetSelectedStyle.Render(lines[i])
		} else {
			lines[i] = presetStyle.Render(lines[i])
		}
	}
	c.list.SetContent(strings.Join(lines, "\n"))
	c.ensureSelectionVisible()
	return cleanerLeftColStyle.Width(totalWidth).Render(c.list.View())
}

func (c *cleanerPage) viewDetail(width int) string {
	if width < 24 {
		width = 24
	}
	var detail []string
	detail = append(detail, sectionTitleStyle.Render("Details"))

	if c.selected >= 0 && c.selected < len(c.presets) {
		p := c.presets[c.selected]
		detail = append(detail, fmt.Sprintf("Name: %s", p.Name))
		detail = append(detail, fmt.Sprintf("Pattern: %s", p.Pattern))
		kind := "Custom"
		if p.BuiltIn {
			kind = "Default"
		}
		detail = append(detail, fmt.Sprintf("Type: %s", kind))
	} else {
		detail = append(detail, "Select a preset to see details.")
	}

	detail = append(detail, "")

	if c.inputMode != cleanerInputNone {
		detail = append(detail, sectionTitleStyle.Render("Edit Preset"))
		detail = append(detail, "Name:")
		detail = append(detail, ui.InputBoxStyle.Width(width-6).Render(c.inputName.View()))
		detail = append(detail, "Pattern:")
		detail = append(detail, ui.InputBoxStyle.Width(width-6).Render(c.inputPath.View()))
		detail = append(detail, "Enter: save · Esc: cancel · Tab: switch field")
	} else {
		detail = append(detail, sectionTitleStyle.Render("Actions"))
		c.setHelpWidth(width)
		h := help.New()
		h.SetWidth(c.helpWidth)
		detail = append(detail, h.ShortHelpView(c.keys.ShortHelp()))
	}

	detail = append(detail, "")

	elapsed := c.elapsed
	if !c.running && c.lastDuration > 0 {
		elapsed = c.lastDuration
	}
	detail = append(detail, fmt.Sprintf("Elapsed: %s", elapsed.Truncate(10*time.Millisecond)))

	if c.running {
		detail = append(detail, c.progress.View())
		total := c.totalItems
		if total == 0 {
			total = len(c.workQueue)
		}
		detail = append(detail, fmt.Sprintf("Progress: %d/%d", c.processed, total))
	}

	return cleanerRightColStyle.Width(width).Render(strings.Join(detail, "\n"))
}

// --- layout helpers ---

func (c *cleanerPage) setHelpWidth(total int) {
	usable := total - cleanerColStyle.GetHorizontalFrameSize()
	if usable < 16 {
		usable = 16
	}
	c.helpWidth = usable
}

func (c *cleanerPage) ensureSelectionVisible() {
	height := c.list.Height()
	if height <= 0 {
		height = 16
	}
	if c.selected < 0 {
		return
	}
	top := c.list.YOffset()
	bottom := top + height - 1
	if c.selected < top {
		c.list.SetYOffset(c.selected)
	} else if c.selected > bottom {
		c.list.SetYOffset(c.selected - height + 1)
	}
}

func (c *cleanerPage) progressPercent(index, totalPresets int) float64 {
	total := c.totalItems
	if total > 0 {
		if c.processed <= 0 {
			return 0
		}
		return float64(c.processed) / float64(total)
	}
	if totalPresets == 0 {
		return 0
	}
	return float64(index+1) / float64(totalPresets)
}

func (c cleanerPage) ShortHelp() []key.Binding  { return c.keys.ShortHelp() }
func (c cleanerPage) FullHelp() [][]key.Binding { return c.keys.FullHelp() }

// --- message types ---

type cleanerTickMsg struct{}

type cleanerStepMsg struct {
	Index  int
	Total  int
	Preset string
	Err    error
	Count  int
}

type cleanerDoneMsg struct {
	Duration time.Duration
	Errors   []string
}

type cleanerLoadedMsg struct {
	Custom []cleanerPreset
	Err    error
}

type cleanerSavedMsg struct{ Err error }

// --- styles & layout constants ---

var (
	presetStyle          = lipgloss.NewStyle()
	presetSelectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(ui.HighlightColor)
	cleanerColStyle      = lipgloss.NewStyle().Padding(0, 1, 0, 1)
	cleanerLeftColStyle  = cleanerColStyle
	cleanerRightColStyle = cleanerColStyle
)

const (
	cleanerColWidth  = 48
	cleanerColHeight = 16
	cleanerGapWidth  = 2
)

func cleanerListWidth() int {
	width := cleanerColWidth - cleanerColStyle.GetHorizontalPadding()
	if width < 16 {
		width = 16
	}
	return width
}
