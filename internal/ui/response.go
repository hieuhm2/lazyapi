package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

type ResponseTab int

const (
	ResponseTabBody ResponseTab = iota
	ResponseTabHeaders
	ResponseTabRaw
)

func (t ResponseTab) String() string {
	switch t {
	case ResponseTabBody:
		return "Body"
	case ResponseTabHeaders:
		return "Headers"
	case ResponseTabRaw:
		return "Raw"
	default:
		return ""
	}
}

type ResponsePanel struct {
	Response  *storage.Response
	ActiveTab ResponseTab
	Viewport  viewport.Model
	Loading   bool
	Focused   bool
	Width     int
	Height    int
}

func NewResponsePanel() ResponsePanel {
	vp := viewport.New(0, 0)
	return ResponsePanel{Viewport: vp}
}

func (p *ResponsePanel) SetResponse(r *storage.Response) {
	p.Response = r
	p.Loading = false
	p.updateViewport()
}

func (p *ResponsePanel) SetLoading() {
	p.Loading = true
}

func (p *ResponsePanel) NextTab() {
	p.ActiveTab = ResponseTab((int(p.ActiveTab) + 1) % 3)
	p.updateViewport()
}

func (p *ResponsePanel) updateViewport() {
	if p.Response == nil {
		return
	}
	content := p.currentTabContent()
	p.Viewport.SetContent(content)
}

func (p ResponsePanel) currentTabContent() string {
	if p.Response == nil {
		return ""
	}
	switch p.ActiveTab {
	case ResponseTabBody:
		return p.Response.Body
	case ResponseTabHeaders:
		var sb strings.Builder
		for k, vals := range p.Response.Headers {
			key := lipgloss.NewStyle().Foreground(ColorMethodGET).Render(k)
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, strings.Join(vals, ", ")))
		}
		return sb.String()
	case ResponseTabRaw:
		return fmt.Sprintf("HTTP/1.1 %s\n\n%s", p.Response.Status, p.Response.Body)
	}
	return ""
}

func (p ResponsePanel) View() string {
	panelStyle := PanelStyle
	titleStyle := TitleStyle
	if p.Focused {
		panelStyle = PanelFocusedStyle
		titleStyle = TitleFocusedStyle
	}

	_ = titleStyle

	var sb strings.Builder

	// Status bar
	if p.Loading {
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render("  Sending request...") + "\n")
	} else if p.Response != nil {
		statusColor := ColorSuccess
		if p.Response.StatusCode >= 400 {
			statusColor = ColorError
		} else if p.Response.StatusCode >= 300 {
			statusColor = ColorWarning
		}
		status := lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(p.Response.Status)
		duration := MutedStyle.Render(fmt.Sprintf("  %dms", p.Response.DurationMs))
		size := MutedStyle.Render(fmt.Sprintf("  %s", formatBytes(p.Response.SizeBytes)))
		sb.WriteString(fmt.Sprintf(" %s%s%s\n", status, duration, size))
	} else {
		sb.WriteString(MutedStyle.Padding(0, 1).Render("Response will appear here") + "\n")
	}

	// Tabs
	tabs := []ResponseTab{ResponseTabBody, ResponseTabHeaders, ResponseTabRaw}
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

	// Content
	if p.Response != nil {
		p.Viewport.Width = p.Width - 6
		p.Viewport.Height = p.Height - 10
		sb.WriteString(p.Viewport.View())
	}

	return panelStyle.
		Width(p.Width - 2).
		Height(p.Height - 2).
		Render(sb.String())
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}
