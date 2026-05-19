package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderHelp(width, height int) string {
	type entry struct{ key, desc string }
	sections := []struct {
		title   string
		entries []entry
	}{
		{"Navigation", []entry{
			{"Tab / Shift+Tab", "Cycle panels"},
			{"1 / 2 / 3 / 4", "Jump to panel"},
			{"h j k l", "Navigate within panel"},
			{"gg / G", "Top / Bottom of list"},
			{"Ctrl+d / Ctrl+u", "Half-page down / up"},
			{"l / h", "Move focus right / left"},
		}},
		{"Actions", []entry{
			{"r / Enter", "Execute request"},
			{"n", "New collection or request"},
			{"d", "Delete selected item"},
			{"e", "Rename selected item"},
			{"m", "Cycle HTTP method (editor focused)"},
			{"t / T", "Next / Prev editor tab"},
			{"E", "Cycle environment"},
		}},
		{"Editor", []entry{
			{"i", "Insert mode (edit URL)"},
			{"e (Body tab)", "Open body in $EDITOR"},
			{"Esc", "Return to Normal mode"},
		}},
		{"Response", []entry{
			{"t / T", "Next / Prev response tab"},
			{"j / k", "Scroll body"},
			{"Ctrl+d/u", "Scroll half-page"},
		}},
		{"Search", []entry{
			{"/", "Search in focused list"},
			{"Esc", "Clear search"},
		}},
		{"App", []entry{
			{"?", "Toggle this help"},
			{"q / Ctrl+c", "Quit"},
		}},
	}

	keyStyle := lipgloss.NewStyle().Foreground(ColorBorderFocused).Bold(true).Width(24)
	descStyle := lipgloss.NewStyle().Foreground(ColorText)
	sectionStyle := lipgloss.NewStyle().Foreground(ColorTitleFocused).Bold(true).MarginTop(1)

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorTitleFocused).Bold(true).Render(" LazyAPI — Keyboard Reference "))
	lines = append(lines, MutedStyle.Render(strings.Repeat("─", 44)))

	for _, s := range sections {
		lines = append(lines, sectionStyle.Render(" "+s.title))
		for _, e := range s.entries {
			row := keyStyle.Render("  "+e.key) + descStyle.Render(e.desc)
			lines = append(lines, row)
		}
	}

	lines = append(lines, "")
	lines = append(lines, MutedStyle.Render("  Press any key to close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderFocused).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
