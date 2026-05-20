package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

type RequestsPanel struct {
	Cursor              int
	ScrollOffset        int
	Filter              string
	SearchActive        bool
	SearchView          string
	GlobalSearchResults []GlobalRequest
	Focused             bool
	Width               int
	Height              int
}

func (p *RequestsPanel) SetFilter(f string) {
	p.Filter = f
	p.Cursor = 0
	p.ScrollOffset = 0
	if f == "" {
		p.GlobalSearchResults = nil
	}
}

// SelectedGlobal returns the currently highlighted global search result, or nil.
func (p RequestsPanel) SelectedGlobal() *GlobalRequest {
	if len(p.GlobalSearchResults) == 0 || p.Cursor >= len(p.GlobalSearchResults) {
		return nil
	}
	return &p.GlobalSearchResults[p.Cursor]
}

func (p RequestsPanel) filteredLen(reqs []storage.Request) int {
	if p.Filter != "" && len(p.GlobalSearchResults) > 0 {
		return len(p.GlobalSearchResults)
	}
	return len(p.Filtered(reqs))
}

func (p RequestsPanel) Filtered(reqs []storage.Request) []storage.Request {
	if p.Filter == "" {
		return reqs
	}
	// In global-search mode Filtered is not used for display; keep for fallback.
	var scored []struct {
		r     storage.Request
		score int
	}
	for _, r := range reqs {
		s := scoreBest(
			fuzzyScore(p.Filter, r.Name),
			fuzzyScore(p.Filter, r.Method),
			fuzzyScore(p.Filter, r.URL),
		)
		if s >= 0 {
			scored = append(scored, struct {
				r     storage.Request
				score int
			}{r, s})
		}
	}
	// sort by score descending
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0 && scored[j].score > scored[j-1].score; j-- {
			scored[j], scored[j-1] = scored[j-1], scored[j]
		}
	}
	out := make([]storage.Request, len(scored))
	for i, s := range scored {
		out[i] = s.r
	}
	return out
}

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

func (p *RequestsPanel) listRows() int {
	if p.Height == 0 {
		return 10
	}
	r := p.Height - 4
	if p.SearchActive {
		r -= 2
	}
	if r < 1 {
		r = 1
	}
	return r
}

func (p *RequestsPanel) clampScroll() {
	rows := p.listRows()
	if p.Cursor < p.ScrollOffset {
		p.ScrollOffset = p.Cursor
	}
	if p.Cursor >= p.ScrollOffset+rows {
		p.ScrollOffset = p.Cursor - rows + 1
	}
	if p.ScrollOffset < 0 {
		p.ScrollOffset = 0
	}
}

func (p *RequestsPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
	p.clampScroll()
}

func (p *RequestsPanel) MoveDown(reqs []storage.Request) {
	if p.Cursor < p.filteredLen(reqs)-1 {
		p.Cursor++
	}
	p.clampScroll()
}

func (p *RequestsPanel) GoTop() {
	p.Cursor = 0
	p.ScrollOffset = 0
}

func (p *RequestsPanel) GoBottom(reqs []storage.Request) {
	n := p.filteredLen(reqs)
	if n > 0 {
		p.Cursor = n - 1
	}
	p.clampScroll()
}

func (p RequestsPanel) View(reqs []storage.Request) string {
	style := PanelStyle
	titleSt := TitleStyle
	if p.Focused {
		style = PanelFocusedStyle
		titleSt = TitleFocusedStyle
	}

	title := titleSt.Render("Requests")
	itemW := p.Width - 6
	rows := p.listRows()

	// Decide whether to render from global search results or local filtered list.
	useGlobal := p.Filter != "" && len(p.GlobalSearchResults) > 0
	var filtered []storage.Request
	var count int
	if useGlobal {
		count = len(p.GlobalSearchResults)
	} else {
		filtered = p.Filtered(reqs)
		count = len(filtered)
	}

	var sb strings.Builder
	sb.WriteString(title + "\n")

	if count == 0 {
		if p.Filter != "" {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No matches") + "\n")
		} else if len(reqs) == 0 {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No requests") + "\n")
		}
	}

	end := p.ScrollOffset + rows
	if end > count {
		end = count
	}
	for i := p.ScrollOffset; i < end; i++ {
		var method, name, breadcrumb string
		if useGlobal {
			gr := p.GlobalSearchResults[i]
			method, name, breadcrumb = gr.Req.Method, gr.Req.Name, gr.ColBreadcrumb
		} else {
			r := filtered[i]
			method, name = r.Method, r.Name
		}

		methodStr := MethodStyle(method).Render(method)

		if i == p.Cursor && p.Focused {
			sb.WriteString(SelectedItemStyle.Width(itemW).Render(methodStr+" "+name) + "\n")
		} else if i == p.Cursor {
			line := methodStr + " " + name
			if breadcrumb != "" {
				line += "  " + breadcrumb
			}
			sb.WriteString(lipgloss.NewStyle().Foreground(ColorSelectedText).Width(itemW).Padding(0, 1).Render(line) + "\n")
		} else {
			line := methodStr + " " + name
			if breadcrumb != "" {
				line += MutedStyle.Render("  " + breadcrumb)
			}
			sb.WriteString(NormalItemStyle.Width(itemW).Render(line) + "\n")
		}
	}

	// In-panel search bar
	if p.SearchActive {
		sb.WriteString("\n")
		sep := lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", p.Width-6))
		prefix := lipgloss.NewStyle().Foreground(ColorBorderFocused).Bold(true).Render("/")
		sb.WriteString(sep + "\n")
		sb.WriteString(prefix + " " + p.SearchView + "\n")
	}

	return style.Width(p.Width - 2).Height(p.Height - 2).Render(sb.String())
}
