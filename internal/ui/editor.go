package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

type EditorTab int

const (
	EditorTabHeaders EditorTab = iota
	EditorTabBody
	EditorTabAuth
)

func (t EditorTab) String() string {
	switch t {
	case EditorTabHeaders:
		return "Headers"
	case EditorTabBody:
		return "Body"
	case EditorTabAuth:
		return "Auth"
	default:
		return ""
	}
}

type EditorPanel struct {
	Request    *storage.Request
	ActiveTab  EditorTab
	URLInput   textinput.Model
	BodyInput  textinput.Model
	Focused    bool
	InsertMode bool
	Width      int
	Height     int
}

func NewEditorPanel() EditorPanel {
	url := textinput.New()
	url.Placeholder = "https://api.example.com/endpoint"
	url.CharLimit = 512

	body := textinput.New()
	body.Placeholder = `{"key": "value"}`
	body.CharLimit = 4096

	return EditorPanel{
		URLInput:  url,
		BodyInput: body,
	}
}

func (p *EditorPanel) SetRequest(r *storage.Request) {
	p.Request = r
	if r != nil {
		p.URLInput.SetValue(r.URL)
		p.BodyInput.SetValue(r.Body)
	} else {
		p.URLInput.SetValue("")
		p.BodyInput.SetValue("")
	}
}

func (p *EditorPanel) NextTab() {
	p.ActiveTab = EditorTab((int(p.ActiveTab) + 1) % 3)
}

func (p EditorPanel) View() string {
	panelStyle := PanelStyle
	titleStyle := TitleStyle
	if p.Focused {
		panelStyle = PanelFocusedStyle
		titleStyle = TitleFocusedStyle
	}

	title := titleStyle.Render("Request Editor")

	var sb strings.Builder
	sb.WriteString(title + "\n")

	if p.Request == nil {
		sb.WriteString(MutedStyle.Padding(1, 1).Render("Select a request to edit"))
		return panelStyle.
			Width(p.Width - 2).
			Height(p.Height - 2).
			Render(sb.String())
	}

	// Method + URL row
	method := MethodStyle(p.Request.Method).
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Render(p.Request.Method)

	urlWidth := p.Width - lipgloss.Width(method) - 8
	p.URLInput.Width = urlWidth

	urlBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Width(urlWidth).
		Render(p.URLInput.View())

	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, method, " ", urlBox) + "\n\n")

	// Tabs
	tabs := []EditorTab{EditorTabHeaders, EditorTabBody, EditorTabAuth}
	tabBar := ""
	for _, t := range tabs {
		label := fmt.Sprintf(" %s ", t.String())
		if t == p.ActiveTab {
			tabBar += lipgloss.NewStyle().
				Foreground(ColorTitleFocused).
				Bold(true).
				Underline(true).
				Render(label)
		} else {
			tabBar += MutedStyle.Render(label)
		}
		tabBar += " "
	}
	sb.WriteString(tabBar + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", p.Width-6)) + "\n")

	// Tab content
	switch p.ActiveTab {
	case EditorTabHeaders:
		if p.Request.Headers == nil || len(p.Request.Headers) == 0 {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No headers. Press n to add.") + "\n")
		} else {
			for k, v := range p.Request.Headers {
				key := lipgloss.NewStyle().Foreground(ColorMethodGET).Render(k)
				val := NormalItemStyle.Render(v)
				sb.WriteString(fmt.Sprintf("  %s: %s\n", key, val))
			}
		}
	case EditorTabBody:
		p.BodyInput.Width = p.Width - 8
		sb.WriteString(lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			Width(p.Width - 8).
			Render(p.BodyInput.View()) + "\n")
	case EditorTabAuth:
		sb.WriteString(MutedStyle.Padding(0, 1).Render("Auth not configured") + "\n")
	}

	return panelStyle.
		Width(p.Width - 2).
		Height(p.Height - 2).
		Render(sb.String())
}
