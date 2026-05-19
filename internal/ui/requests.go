package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

type RequestsPanel struct {
	Requests []storage.Request
	Cursor   int
	Focused  bool
	Width    int
	Height   int
}

func NewRequestsPanel() RequestsPanel {
	return RequestsPanel{}
}

func (p *RequestsPanel) SetRequests(reqs []storage.Request) {
	p.Requests = reqs
	if p.Cursor >= len(reqs) {
		p.Cursor = 0
	}
}

func (p *RequestsPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

func (p *RequestsPanel) MoveDown() {
	if p.Cursor < len(p.Requests)-1 {
		p.Cursor++
	}
}

func (p *RequestsPanel) GoTop() {
	p.Cursor = 0
}

func (p *RequestsPanel) GoBottom() {
	if len(p.Requests) > 0 {
		p.Cursor = len(p.Requests) - 1
	}
}

func (p *RequestsPanel) Selected() *storage.Request {
	if len(p.Requests) == 0 {
		return nil
	}
	return &p.Requests[p.Cursor]
}

func (p RequestsPanel) View() string {
	panelStyle := PanelStyle
	titleStyle := TitleStyle
	if p.Focused {
		panelStyle = PanelFocusedStyle
		titleStyle = TitleFocusedStyle
	}

	title := titleStyle.Render("Requests")
	inner := lipgloss.NewStyle().Width(p.Width - 4).Height(p.Height - 4)

	content := title + "\n"

	if len(p.Requests) == 0 {
		content += MutedStyle.Padding(0, 1).Render("No requests") + "\n"
	}

	for i, r := range p.Requests {
		method := MethodStyle(r.Method).Render(r.Method)
		name := r.Name
		line := method + " " + name

		if i == p.Cursor && p.Focused {
			content += SelectedItemStyle.Width(p.Width - 6).Render(line) + "\n"
		} else if i == p.Cursor {
			content += lipgloss.NewStyle().
				Foreground(ColorSelectedText).
				Width(p.Width - 6).
				Padding(0, 1).
				Render(line) + "\n"
		} else {
			content += NormalItemStyle.Width(p.Width - 6).Render(line) + "\n"
		}
	}

	return panelStyle.
		Width(p.Width - 2).
		Height(p.Height - 2).
		Render(inner.Render(content))
}
