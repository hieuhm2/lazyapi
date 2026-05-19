package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

type RequestsPanel struct {
	Cursor  int
	Filter  string
	Focused bool
	Width   int
	Height  int
}

func (p *RequestsPanel) SetFilter(f string) {
	p.Filter = f
	p.Cursor = 0
}

func (p RequestsPanel) Filtered(reqs []storage.Request) []storage.Request {
	if p.Filter == "" {
		return reqs
	}
	f := strings.ToLower(p.Filter)
	var out []storage.Request
	for _, r := range reqs {
		if strings.Contains(strings.ToLower(r.Name), f) ||
			strings.Contains(strings.ToLower(r.URL), f) ||
			strings.Contains(strings.ToLower(r.Method), f) {
			out = append(out, r)
		}
	}
	return out
}

// SelectedIdx returns the index in the original slice of the cursor item.
func (p RequestsPanel) SelectedIdx(reqs []storage.Request) int {
	filtered := p.Filtered(reqs)
	if len(filtered) == 0 || p.Cursor >= len(filtered) {
		return -1
	}
	target := filtered[p.Cursor].ID
	for i, r := range reqs {
		if r.ID == target {
			return i
		}
	}
	return -1
}

func (p *RequestsPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

func (p *RequestsPanel) MoveDown(reqs []storage.Request) {
	max := len(p.Filtered(reqs)) - 1
	if p.Cursor < max {
		p.Cursor++
	}
}

func (p *RequestsPanel) GoTop() { p.Cursor = 0 }

func (p *RequestsPanel) GoBottom(reqs []storage.Request) {
	n := len(p.Filtered(reqs))
	if n > 0 {
		p.Cursor = n - 1
	}
}

func (p RequestsPanel) View(reqs []storage.Request) string {
	style := PanelStyle
	titleSt := TitleStyle
	if p.Focused {
		style = PanelFocusedStyle
		titleSt = TitleFocusedStyle
	}

	filtered := p.Filtered(reqs)
	title := titleSt.Render("Requests")
	if p.Filter != "" {
		title = titleSt.Render(fmt.Sprintf("Requests /%s", p.Filter))
	}

	var sb strings.Builder
	sb.WriteString(title + "\n")

	if len(filtered) == 0 {
		if len(reqs) == 0 {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No requests") + "\n")
		} else {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No matches") + "\n")
		}
	}

	for i, r := range filtered {
		method := MethodStyle(r.Method).Render(r.Method)
		line := method + " " + r.Name
		itemW := p.Width - 6
		if i == p.Cursor && p.Focused {
			sb.WriteString(SelectedItemStyle.Width(itemW).Render(method+" "+r.Name) + "\n")
		} else if i == p.Cursor {
			sb.WriteString(lipgloss.NewStyle().Foreground(ColorSelectedText).Width(itemW).Padding(0, 1).Render(line) + "\n")
		} else {
			sb.WriteString(NormalItemStyle.Width(itemW).Render(line) + "\n")
		}
	}

	return style.Width(p.Width - 2).Height(p.Height - 2).Render(sb.String())
}
