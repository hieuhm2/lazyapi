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

var editorTabs = []EditorTab{EditorTabHeaders, EditorTabBody, EditorTabAuth}

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

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

type EditorPanel struct {
	Request    *storage.Request
	ActiveTab  EditorTab
	URLInput   textinput.Model
	InsertMode bool
	Focused    bool
	Width      int
	Height     int
}

func NewEditorPanel() EditorPanel {
	url := textinput.New()
	url.Placeholder = "https://api.example.com/endpoint"
	url.CharLimit = 512

	return EditorPanel{URLInput: url}
}

func (p *EditorPanel) SetRequest(r *storage.Request) {
	p.Request = r
	if r != nil {
		p.URLInput.SetValue(r.URL)
	} else {
		p.URLInput.SetValue("")
	}
}

func (p *EditorPanel) NextTab() {
	p.ActiveTab = EditorTab((int(p.ActiveTab) + 1) % len(editorTabs))
}

func (p *EditorPanel) PrevTab() {
	p.ActiveTab = EditorTab((int(p.ActiveTab) - 1 + len(editorTabs)) % len(editorTabs))
}

func (p *EditorPanel) CycleMethod() {
	if p.Request == nil {
		return
	}
	for i, m := range methods {
		if m == p.Request.Method {
			p.Request.Method = methods[(i+1)%len(methods)]
			return
		}
	}
	p.Request.Method = methods[0]
}

func (p EditorPanel) View() string {
	style := PanelStyle
	titleSt := TitleStyle
	if p.Focused {
		style = PanelFocusedStyle
		titleSt = TitleFocusedStyle
	}

	title := titleSt.Render("Request Editor")
	var sb strings.Builder
	sb.WriteString(title + "\n")

	if p.Request == nil {
		sb.WriteString(MutedStyle.Padding(1, 1).Render("Select a request"))
		return style.Width(p.Width - 2).Height(p.Height - 2).Render(sb.String())
	}

	// Method + URL
	methodBox := MethodStyle(p.Request.Method).
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Render(p.Request.Method)

	methodW := lipgloss.Width(methodBox)
	urlAvail := p.Width - methodW - 10
	if urlAvail < 10 {
		urlAvail = 10
	}
	p.URLInput.Width = urlAvail

	urlBorderColor := ColorBorder
	if p.InsertMode {
		urlBorderColor = ColorBorderFocused
	}
	urlBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(urlBorderColor).
		Padding(0, 1).
		Width(urlAvail).
		Render(p.URLInput.View())

	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, methodBox, " ", urlBox) + "\n\n")

	// Tab bar
	tabBar := ""
	for _, t := range editorTabs {
		label := fmt.Sprintf(" %s ", t.String())
		if t == p.ActiveTab {
			tabBar += lipgloss.NewStyle().Foreground(ColorTitleFocused).Bold(true).Underline(true).Render(label)
		} else {
			tabBar += MutedStyle.Render(label)
		}
		tabBar += " "
	}
	sb.WriteString(tabBar + "\n")
	sb.WriteString(MutedStyle.Render(strings.Repeat("─", p.Width-6)) + "\n")

	// Tab content
	contentW := p.Width - 8
	switch p.ActiveTab {
	case EditorTabHeaders:
		if len(p.Request.Headers) == 0 {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No headers  ·  press n to add") + "\n")
		} else {
			for k, v := range p.Request.Headers {
				key := lipgloss.NewStyle().Foreground(ColorMethodGET).Render(k)
				sb.WriteString(fmt.Sprintf("  %s: %s\n", key, v))
			}
		}
	case EditorTabBody:
		if p.Request.Body == "" {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("Empty body  ·  press e to edit in $EDITOR") + "\n")
		} else {
			preview := p.Request.Body
			if len(preview) > 300 {
				preview = preview[:300] + "..."
			}
			sb.WriteString(lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1).
				Width(contentW).
				Render(preview) + "\n")
		}
		hint := MutedStyle.Render("  e · edit in $EDITOR")
		sb.WriteString(hint + "\n")
	case EditorTabAuth:
		sb.WriteString(MutedStyle.Padding(0, 1).Render("No auth configured") + "\n")
	}

	return style.Width(p.Width - 2).Height(p.Height - 2).Render(sb.String())
}
