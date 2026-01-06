package pages

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"itzdabbzz.me/gomctools/internal/ui"
)

type cleanerInputMode int

const (
	cleanerInputNone cleanerInputMode = iota
	cleanerInputAdd
	cleanerInputEdit
)

type cleanerPreset struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Enabled bool   `json:"enabled"`
	BuiltIn bool   `json:"-"`
}

type cleanerKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Toggle key.Binding
	Clean  key.Binding
	New    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Save   key.Binding
	Help   key.Binding
}

func (k cleanerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Toggle, k.Clean, k.Help}
}

func (k cleanerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Toggle, k.Clean},
		{k.New, k.Edit, k.Delete, k.Save, k.Help},
	}
}

func defaultCleanerKeyMap() cleanerKeyMap {
	return cleanerKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "select previous"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "select next"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle preset"),
		),
		Clean: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "run cleaner"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new preset"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit preset"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete preset"),
		),
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save presets"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
	}
}

type cleanerConfig struct {
	Custom []cleanerPreset `json:"custom_presets"`
}

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

	pageWidth  int
	pageHeight int

	saving bool
}

func NewCleanerPage(state *ui.SharedState) ui.Page {
	nameInput := textinput.New()
	nameInput.Placeholder = "Preset name"
	nameInput.CharLimit = 64
	nameInput.Prompt = ""

	pathInput := textinput.New()
	pathInput.Placeholder = "Path or folder"
	pathInput.CharLimit = 256
	pathInput.Prompt = ""

	pr := progress.New(progress.WithSolidFill(ui.HighlightColor.Light))
	pr.Width = 32

	list := viewport.New(cleanerColWidth, cleanerColHeight)
	list.MouseWheelEnabled = true
	list.MouseWheelDelta = 2

	return &cleanerPage{
		state:      state,
		keys:       defaultCleanerKeyMap(),
		presets:    defaultCleanerPresets(),
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

// SetZone wires bubblezone; cleaner currently keyboard-driven but keeps parity with other pages.
func (c *cleanerPage) SetZone(z *zone.Manager, prefix string) {
	c.zone = z
	c.prefix = prefix
}

func (c *cleanerPage) Title() string { return "Pack Cleaner" }
func (c *cleanerPage) Init() tea.Cmd { return nil }

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
		return c, func() tea.Msg { return cleanerDoneMsg{Duration: time.Since(c.workStart), Errors: c.workErrors} }
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
		c.pageWidth = m.Width
		c.pageHeight = m.Height

		// Reserve headroom for tabs, footer, the status line, and a spacer so the list never overflows the window.
		reserved := ui.DocStyle.GetVerticalFrameSize() + ui.WindowStyle.GetVerticalFrameSize() + 8
		heightBudget := m.Height - reserved
		if heightBudget < cleanerColHeight {
			heightBudget = cleanerColHeight
		}
		c.list.Width = cleanerListWidth()
		c.list.Height = heightBudget
		c.ensureSelectionVisible()

		contentWidth := c.availableContentWidth()
		rightWidth := contentWidth - cleanerColWidth - cleanerGapWidth
		if rightWidth < 32 {
			rightWidth = 32
		}
		c.setHelpWidth(rightWidth)
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
	contentWidth := c.availableContentWidth()
	leftTotal := cleanerColWidth
	rightTotal := contentWidth - leftTotal - cleanerGapWidth
	if rightTotal < 32 {
		rightTotal = 32
	}
	left := c.viewPresetList(leftTotal)
	right := c.viewDetail(rightTotal)
	spacer := strings.Repeat(" ", cleanerGapWidth)

	row := lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
	body := lipgloss.NewStyle().Width(contentWidth).Render(row)
	if c.status != "" {
		body += "\n" + statusStyle.Render(c.status)
	}
	return body
}

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

	// pre-count items for progress
	total := 0
	for _, p := range enabled {
		count, _ := countEntries(c.state.Pack.MinecraftDir, p)
		total += count
	}
	if total == 0 {
		total = len(enabled) // fallback to preset-based progress
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

func deletePreset(root string, preset cleanerPreset) (int, error) {
	if root == "" {
		return 0, errors.New("missing root path")
	}

	pattern := strings.TrimSpace(preset.Pattern)
	if pattern == "" {
		return 0, nil
	}
	pattern = normalizePattern(pattern)

	rel := strings.TrimPrefix(pattern, string(os.PathSeparator))
	target := filepath.Clean(filepath.Join(root, rel))

	if !strings.HasPrefix(target, filepath.Clean(root)) {
		return 0, fmt.Errorf("refusing to delete outside root: %s", target)
	}
	if target == filepath.Clean(root) {
		return 0, fmt.Errorf("refusing to delete root directory")
	}

	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	count := 1
	if info.IsDir() {
		c, cerr := countDirEntries(target)
		if cerr == nil {
			count = c
		}
		return count, os.RemoveAll(target)
	}
	return count, os.Remove(target)
}

func (c *cleanerPage) loadCustomPresetsCmd(root string) tea.Cmd {
	return func() tea.Msg {
		custom, err := readCleanerConfig(root)
		return cleanerLoadedMsg{Custom: custom, Err: err}
	}
}

func (c *cleanerPage) saveCustomPresetsCmd() tea.Cmd {
	if c.state == nil || c.state.Pack.MinecraftDir == "" {
		c.status = "Load a pack before saving presets."
		return nil
	}
	c.saving = true
	root := c.state.Pack.MinecraftDir
	presets := filterCustom(c.presets)
	return func() tea.Msg {
		err := writeCleanerConfig(root, presets)
		return cleanerSavedMsg{Err: err}
	}
}

func readCleanerConfig(root string) ([]cleanerPreset, error) {
	if root == "" {
		return nil, errors.New("missing root path")
	}
	path := filepath.Join(root, "gomctools.cleaner.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg cleanerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return normalizeCustom(cfg.Custom), nil
}

func countEntries(root string, preset cleanerPreset) (int, error) {
	target := filepath.Clean(filepath.Join(root, normalizePattern(preset.Pattern)))
	if !strings.HasPrefix(target, filepath.Clean(root)) {
		return 0, fmt.Errorf("outside root")
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	if info.IsDir() {
		return countDirEntries(target)
	}
	return 1, nil
}

func countDirEntries(dir string) (int, error) {
	count := 0
	walkErr := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		count++
		return nil
	})
	if walkErr != nil {
		return count, walkErr
	}
	if count == 0 {
		count = 1 // empty dirs still count as one unit of work
	}
	return count, nil
}

func writeCleanerConfig(root string, presets []cleanerPreset) error {
	if root == "" {
		return errors.New("missing root path")
	}
	path := filepath.Join(root, "gomctools.cleaner.json")
	cfg := cleanerConfig{Custom: normalizeCustom(presets)}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func normalizePattern(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, string(os.PathSeparator))
	return p
}

func normalizeCustom(list []cleanerPreset) []cleanerPreset {
	out := make([]cleanerPreset, 0, len(list))
	for _, p := range list {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Pattern) == "" {
			continue
		}
		p.BuiltIn = false
		p.Pattern = normalizePattern(p.Pattern)
		out = append(out, p)
	}
	return out
}

func filterCustom(list []cleanerPreset) []cleanerPreset {
	out := []cleanerPreset{}
	for _, p := range list {
		if p.BuiltIn {
			continue
		}
		out = append(out, p)
	}
	return out
}

func defaultCleanerPresets() []cleanerPreset {
	names := []string{
		".cache/",
		".mixin.out/",
		".probe/",
		".vscode/",
		"bluemap/",
		"cachecoremods/",
		"crash-reports/",
		"data/",
		"Distant_Horizons_server_data/",
		"downloads/",
		"dynamic-data-pack-cache/",
		"dynamic-resource-pack-cache/",
		"fancymenu_data/",
		"journeymap/",
		"local/",
		"logs/",
		"midi_files/",
		"moddata/",
		"moonlight-global-datapacks/",
		"patchouli_books/",
		"saves/",
		"screenshots/",
		"waypoints/",
		"command_history.txt",
		"patchouli_data.json",
		"servers.dat",
		"servers.dat_old",
		"usercache.json",
		"usernamecache.json",
	}

	presets := make([]cleanerPreset, 0, len(names))
	for _, n := range names {
		presets = append(presets, cleanerPreset{Name: displayName(n), Pattern: normalizePattern(n), Enabled: false, BuiltIn: true})
	}
	return presets
}

func displayName(path string) string {
	trimmed := strings.TrimSuffix(path, string(os.PathSeparator))
	trimmed = strings.TrimPrefix(trimmed, ".")
	if trimmed == "" {
		return path
	}
	return trimmed
}

func (c *cleanerPage) viewPresetList(totalWidth int) string {
	if len(c.presets) == 0 {
		c.list.SetContent("No presets")
		c.list.Width = cleanerListWidth()
		c.list.Height = cleanerColHeight
		return cleanerLeftColStyle.Width(totalWidth).Render(c.list.View())
	}

	lines := make([]string, 0, len(c.presets)+2)
	lines = append(lines, sectionTitleStyle.Render("Presets"))
	for i, p := range c.presets {
		check := checkbox(p.Enabled)
		label := fmt.Sprintf("%s %s", check, p.Name)
		if p.BuiltIn {
			label += " • default"
		}
		wrapWidth := c.list.Width - 2
		if wrapWidth < 8 {
			wrapWidth = c.list.Width
		}
		label = wrapToWidth(label, wrapWidth)
		if i == c.selected {
			label = presetSelectedStyle.Render(label)
		} else {
			label = presetStyle.Render(label)
		}
		lines = append(lines, label)
	}

	content := strings.Join(lines, "\n")
	c.list.Width = cleanerListWidth()
	if c.list.Height == 0 {
		c.list.Height = cleanerColHeight
	}
	c.list.SetContent(content)
	c.ensureSelectionVisible()
	return cleanerLeftColStyle.Width(totalWidth).Render(c.list.View())
}

func (c *cleanerPage) viewDetail(width int) string {
	if width < cleanerColWidth {
		width = cleanerColWidth
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
		detail = append(detail, ui.RenderShortHelp(c.keys.ShortHelp()))
	}

	detail = append(detail, "")

	elapsed := c.elapsed
	if !c.running && c.lastDuration > 0 {
		elapsed = c.lastDuration
	}

	timerLine := fmt.Sprintf("Elapsed: %s", elapsed.Truncate(10*time.Millisecond))
	detail = append(detail, timerLine)

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

func (c *cleanerPage) estimatedColumnWidth() int { return cleanerColWidth }

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

func (c *cleanerPage) ensureSelectionVisible() {
	height := c.list.Height
	if height <= 0 {
		height = cleanerColHeight
	}
	if c.selected < 0 {
		return
	}
	top := c.list.YOffset
	bottom := top + height - 1
	if c.selected < top {
		c.list.SetYOffset(c.selected)
	} else if c.selected > bottom {
		c.list.SetYOffset(c.selected - height + 1)
	}
}

func (c *cleanerPage) setHelpWidth(total int) {
	usable := total - cleanerColStyle.GetHorizontalFrameSize()
	if usable < 16 {
		usable = 16
	}
	c.helpWidth = usable
}

func (c *cleanerPage) availableContentWidth() int {
	width := c.pageWidth
	if width == 0 {
		return cleanerColWidth*2 + cleanerColStyle.GetHorizontalFrameSize() + 4
	}
	inner := width - ui.DocStyle.GetHorizontalFrameSize()
	if inner < ui.MinWidth {
		inner = ui.MinWidth
	}
	content := inner - ui.WindowStyle.GetHorizontalFrameSize()
	if content < cleanerColWidth*2 {
		content = cleanerColWidth*2 + cleanerColStyle.GetHorizontalFrameSize()
	}
	return content
}

func wrapToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Width(width).Render(s)
}

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

func (c cleanerPage) ShortHelp() []key.Binding { return c.keys.ShortHelp() }

func (c cleanerPage) FullHelp() [][]key.Binding { return c.keys.FullHelp() }

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
