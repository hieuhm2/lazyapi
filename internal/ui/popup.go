package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

const popupInnerWidth = 46

var (
	popupBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorderFocused).
				Padding(1, 3)

	popupDangerBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorError).
				Padding(1, 3)

	popupTitleStyle = lipgloss.NewStyle().
			Foreground(ColorTitleFocused).
			Bold(true)

	popupInputBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorBorderFocused).
				Padding(0, 1).
				Width(popupInnerWidth)
)

// InputPopup renders a titled text-input popup. Returns an unsized string —
// the caller centers it with PlaceOverlay.
func InputPopup(title, hint string, input textinput.Model) string {
	input.Width = popupInnerWidth - 2

	content := lipgloss.JoinVertical(lipgloss.Left,
		popupTitleStyle.Render(title),
		"",
		popupInputBoxStyle.Render(input.View()),
		"",
		MutedStyle.Render(hint),
	)

	return popupBorderStyle.Width(popupInnerWidth + 6).Render(content)
}

// ConfirmPopup renders a yes/no delete confirmation popup.
func ConfirmPopup(itemName string) string {
	question := lipgloss.NewStyle().Foreground(ColorText).Render("Delete this item?")
	item := lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true).
		Render(`"` + itemName + `"`)

	yKey := lipgloss.NewStyle().Foreground(ColorMethodGET).Bold(true).Render("y")
	nKey := lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render("n / Esc")
	keys := yKey + MutedStyle.Render("  confirm      ") + nKey + MutedStyle.Render("  cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		question,
		"",
		"  "+item,
		"",
		keys,
	)

	return popupDangerBorderStyle.Width(popupInnerWidth + 6).Render(content)
}
