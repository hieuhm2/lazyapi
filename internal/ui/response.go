package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
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
	return ResponsePanel{Viewport: viewport.New(0, 0)}
}

func (p *ResponsePanel) SetResponse(r *storage.Response) {
	p.Response = r
	p.Loading = false
	p.refreshContent()
}

func (p *ResponsePanel) SetLoading() {
	p.Loading = true
}

func (p *ResponsePanel) NextTab() {
	p.ActiveTab = ResponseTab((int(p.ActiveTab) + 1) % 3)
	p.refreshContent()
}

func (p *ResponsePanel) refreshContent() {
	if p.Response == nil {
		return
	}
	p.Viewport.SetContent(p.tabContent())
	p.Viewport.GotoTop()
}

func (p ResponsePanel) tabContent() string {
	if p.Response == nil {
		return ""
	}
	switch p.ActiveTab {
	case ResponseTabBody:
		if p.Response.Error != "" {
			return lipgloss.NewStyle().Foreground(ColorError).Render(p.Response.Error)
		}
		return highlightBody(p.Response.Body, p.Response.ContentType)
	case ResponseTabHeaders:
		var sb strings.Builder
		for k, vals := range p.Response.Headers {
			key := lipgloss.NewStyle().Foreground(ColorMethodGET).Render(k)
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, strings.Join(vals, ", ")))
		}
		return sb.String()
	case ResponseTabRaw:
		var hdr strings.Builder
		for k, vals := range p.Response.Headers {
			hdr.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(vals, ", ")))
		}
		return fmt.Sprintf("HTTP/1.1 %s\n%s\n%s", p.Response.Status, hdr.String(), p.Response.Body)
	}
	return ""
}

func (p ResponsePanel) View() string {
	style := PanelStyle
	if p.Focused {
		style = PanelFocusedStyle
	}

	var sb strings.Builder

	// Status line
	if p.Loading {
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render("  Sending...") + "\n")
	} else if p.Response != nil && p.Response.Error != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render("  Error: "+p.Response.Error) + "\n")
	} else if p.Response != nil {
		statusColor := ColorSuccess
		if p.Response.StatusCode >= 500 {
			statusColor = ColorError
		} else if p.Response.StatusCode >= 400 {
			statusColor = ColorError
		} else if p.Response.StatusCode >= 300 {
			statusColor = ColorWarning
		}
		status := lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(p.Response.Status)
		duration := MutedStyle.Render(fmt.Sprintf("  %dms", p.Response.DurationMs))
		size := MutedStyle.Render(fmt.Sprintf("  %s", formatBytes(p.Response.SizeBytes)))
		sb.WriteString(fmt.Sprintf(" %s%s%s\n", status, duration, size))
	} else {
		sb.WriteString(MutedStyle.Padding(0, 1).Render("Response will appear here  ·  press r to execute") + "\n")
	}

	// Tab bar
	tabs := []ResponseTab{ResponseTabBody, ResponseTabHeaders, ResponseTabRaw}
	tabBar := ""
	for _, t := range tabs {
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

	// Viewport
	if p.Response != nil {
		vpW := p.Width - 6
		vpH := p.Height - 9
		if vpW < 1 {
			vpW = 1
		}
		if vpH < 1 {
			vpH = 1
		}
		p.Viewport.Width = vpW
		p.Viewport.Height = vpH
		sb.WriteString(p.Viewport.View())
	}

	return style.Width(p.Width - 2).Height(p.Height - 2).Render(sb.String())
}

func highlightBody(body, contentType string) string {
	if body == "" {
		return ""
	}
	lang := detectLang(body, contentType)

	// Pretty-print JSON
	if lang == "json" {
		var buf bytes.Buffer
		if err := json.Indent(&buf, []byte(body), "", "  "); err == nil {
			body = buf.String()
		}
	}

	lexer := lexers.Get(lang)
	if lexer == nil {
		return body
	}

	style := styles.Get("monokai")
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return body
	}

	it, err := lexer.Tokenise(nil, body)
	if err != nil {
		return body
	}

	var out strings.Builder
	if err := formatter.Format(&out, style, it); err != nil {
		return body
	}
	return out.String()
}

func detectLang(body, contentType string) string {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "json") {
		return "json"
	}
	if strings.Contains(ct, "xml") {
		return "xml"
	}
	if strings.Contains(ct, "html") {
		return "html"
	}
	// Sniff content
	body = strings.TrimSpace(body)
	if len(body) > 0 && (body[0] == '{' || body[0] == '[') {
		return "json"
	}
	if strings.HasPrefix(body, "<?xml") || strings.HasPrefix(body, "<") {
		return "xml"
	}
	return "text"
}

func formatBytes(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
}
