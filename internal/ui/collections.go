package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

type CollectionsPanel struct {
	Collections []storage.Collection
	Cursor      int
	Focused     bool
	Width       int
	Height      int
}

func NewCollectionsPanel() CollectionsPanel {
	return CollectionsPanel{
		Collections: mockCollections(),
	}
}

func (p *CollectionsPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

func (p *CollectionsPanel) MoveDown() {
	if p.Cursor < len(p.Collections)-1 {
		p.Cursor++
	}
}

func (p *CollectionsPanel) GoTop() {
	p.Cursor = 0
}

func (p *CollectionsPanel) GoBottom() {
	if len(p.Collections) > 0 {
		p.Cursor = len(p.Collections) - 1
	}
}

func (p *CollectionsPanel) Selected() *storage.Collection {
	if len(p.Collections) == 0 {
		return nil
	}
	return &p.Collections[p.Cursor]
}

func (p CollectionsPanel) View() string {
	panelStyle := PanelStyle
	titleStyle := TitleStyle
	if p.Focused {
		panelStyle = PanelFocusedStyle
		titleStyle = TitleFocusedStyle
	}

	title := titleStyle.Render("Collections")
	inner := lipgloss.NewStyle().Width(p.Width - 4).Height(p.Height - 4)

	content := title + "\n"
	for i, c := range p.Collections {
		line := fmt.Sprintf(" %s", c.Name)
		reqCount := MutedStyle.Render(fmt.Sprintf(" (%d)", len(c.Requests)))
		if i == p.Cursor && p.Focused {
			content += SelectedItemStyle.Width(p.Width - 6).Render(line) + "\n"
		} else if i == p.Cursor {
			content += lipgloss.NewStyle().
				Foreground(ColorSelectedText).
				Width(p.Width - 6).
				Padding(0, 1).
				Render(line+reqCount) + "\n"
		} else {
			content += NormalItemStyle.Width(p.Width - 6).Render(line+reqCount) + "\n"
		}
	}

	return panelStyle.
		Width(p.Width - 2).
		Height(p.Height - 2).
		Render(inner.Render(content))
}

func mockCollections() []storage.Collection {
	return []storage.Collection{
		{
			ID:   "1",
			Name: "My API",
			Requests: []storage.Request{
				{ID: "1", Name: "List Users", Method: "GET", URL: "https://jsonplaceholder.typicode.com/users"},
				{ID: "2", Name: "Create User", Method: "POST", URL: "https://jsonplaceholder.typicode.com/users", Body: `{"name":"John","email":"john@example.com"}`},
				{ID: "3", Name: "Update User", Method: "PUT", URL: "https://jsonplaceholder.typicode.com/users/1"},
				{ID: "4", Name: "Delete User", Method: "DELETE", URL: "https://jsonplaceholder.typicode.com/users/1"},
			},
		},
		{
			ID:   "2",
			Name: "Auth",
			Requests: []storage.Request{
				{ID: "5", Name: "Login", Method: "POST", URL: "https://api.example.com/auth/login"},
				{ID: "6", Name: "Refresh Token", Method: "POST", URL: "https://api.example.com/auth/refresh"},
				{ID: "7", Name: "Logout", Method: "POST", URL: "https://api.example.com/auth/logout"},
			},
		},
		{
			ID:   "3",
			Name: "Payments",
			Requests: []storage.Request{
				{ID: "8", Name: "List Transactions", Method: "GET", URL: "https://api.example.com/payments"},
				{ID: "9", Name: "Create Payment", Method: "POST", URL: "https://api.example.com/payments"},
			},
		},
	}
}
