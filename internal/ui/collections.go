package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

type CollectionsPanel struct {
	Cursor  int
	Filter  string
	Focused bool
	Width   int
	Height  int
}

func (p *CollectionsPanel) SetFilter(f string) {
	p.Filter = f
	p.Cursor = 0
}

func (p CollectionsPanel) Filtered(cols []storage.Collection) []storage.Collection {
	if p.Filter == "" {
		return cols
	}
	f := strings.ToLower(p.Filter)
	var out []storage.Collection
	for _, c := range cols {
		if strings.Contains(strings.ToLower(c.Name), f) {
			out = append(out, c)
		}
	}
	return out
}

// SelectedIdx returns the index in the original slice of the cursor item.
func (p CollectionsPanel) SelectedIdx(cols []storage.Collection) int {
	filtered := p.Filtered(cols)
	if len(filtered) == 0 || p.Cursor >= len(filtered) {
		return -1
	}
	target := filtered[p.Cursor].ID
	for i, c := range cols {
		if c.ID == target {
			return i
		}
	}
	return -1
}

func (p *CollectionsPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

func (p *CollectionsPanel) MoveDown(cols []storage.Collection) {
	max := len(p.Filtered(cols)) - 1
	if p.Cursor < max {
		p.Cursor++
	}
}

func (p *CollectionsPanel) GoTop() { p.Cursor = 0 }

func (p *CollectionsPanel) GoBottom(cols []storage.Collection) {
	n := len(p.Filtered(cols))
	if n > 0 {
		p.Cursor = n - 1
	}
}

func (p CollectionsPanel) View(cols []storage.Collection) string {
	style := PanelStyle
	titleSt := TitleStyle
	if p.Focused {
		style = PanelFocusedStyle
		titleSt = TitleFocusedStyle
	}

	filtered := p.Filtered(cols)
	title := titleSt.Render("Collections")
	if p.Filter != "" {
		title = titleSt.Render(fmt.Sprintf("Collections /%s", p.Filter))
	}

	var sb strings.Builder
	sb.WriteString(title + "\n")

	if len(filtered) == 0 {
		sb.WriteString(MutedStyle.Padding(0, 1).Render("No matches") + "\n")
	}

	for i, c := range filtered {
		reqCount := MutedStyle.Render(fmt.Sprintf(" (%d)", len(c.Requests)))
		line := " " + c.Name + reqCount
		itemW := p.Width - 6
		if i == p.Cursor && p.Focused {
			sb.WriteString(SelectedItemStyle.Width(itemW).Render(" "+c.Name) + "\n")
		} else if i == p.Cursor {
			sb.WriteString(lipgloss.NewStyle().Foreground(ColorSelectedText).Width(itemW).Padding(0, 1).Render(line) + "\n")
		} else {
			sb.WriteString(NormalItemStyle.Width(itemW).Render(line) + "\n")
		}
	}

	return style.Width(p.Width - 2).Height(p.Height - 2).Render(sb.String())
}
