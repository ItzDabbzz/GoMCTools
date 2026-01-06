package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	docStyle          = lipgloss.NewStyle().Padding(0, 2, 0, 2)
	highlightColor    = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	inactiveTabStyle  = lipgloss.NewStyle().Border(inactiveTabBorder, true).BorderForeground(highlightColor).Padding(0, 1)
	activeTabStyle    = inactiveTabStyle.Border(activeTabBorder, true)
	windowStyle       = lipgloss.NewStyle().BorderForeground(highlightColor).Padding(1, 0).Align(lipgloss.Center).Border(lipgloss.NormalBorder()).UnsetBorderTop()
	keyLabelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#0b1021")).Background(highlightColor).Padding(0, 1)
	keyDescStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#8f9bb3")).MarginRight(2)
	footerStyle       = lipgloss.NewStyle().MarginTop(1)
	inputBoxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(highlightColor).Padding(0, 1)
	borderFillStyle   = lipgloss.NewStyle().Foreground(highlightColor)
	warningBoxStyle   = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#7a1a1a")).
				Border(lipgloss.RoundedBorder(), true).
				BorderForeground(lipgloss.Color("#ff5f56")).
				Padding(1, 3)
)

// Exported aliases for pages package
var (
	HighlightColor  = highlightColor
	DocStyle        = docStyle
	WindowStyle     = windowStyle
	InputBoxStyle   = inputBoxStyle
	BorderFillStyle = borderFillStyle
)

func renderTabs(z *zone.Manager, prefix string, titles []string, active int) string {
	var renderedTabs []string

	for i, t := range titles {
		style := inactiveTabStyle
		isFirst, isLast, isActive := i == 0, i == len(titles)-1, i == active
		if isActive {
			style = activeTabStyle
		}
		border, _, _, _, _ := style.GetBorder()
		switch {
		case isFirst && isActive:
			border.BottomLeft = "│"
		case isFirst && !isActive:
			border.BottomLeft = "├"
		case isLast && !isActive:
			border.BottomRight = "┴"
		}
		style = style.Border(border)

		rendered := style.Render(t)
		if z != nil {
			id := fmt.Sprintf("%stab-%d", prefix, i)
			rendered = z.Mark(id, rendered)
		}

		renderedTabs = append(renderedTabs, rendered)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
}
