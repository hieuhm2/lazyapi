package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

// ── Tree node ─────────────────────────────────────────────────────────────────

type TreeNode struct {
	Path       []int
	Depth      int
	ID         string
	Name       string
	Breadcrumb string // full ancestor path, set during fuzzy search
	HasKids    bool
	ReqCount   int
}

func BuildTree(cols []storage.Collection, expanded map[string]bool, depth int, prefix []int) []TreeNode {
	var nodes []TreeNode
	for i, col := range cols {
		path := make([]int, len(prefix)+1)
		copy(path, prefix)
		path[len(prefix)] = i

		hasKids := len(col.Children) > 0
		nodes = append(nodes, TreeNode{
			Path:     path,
			Depth:    depth,
			ID:       col.ID,
			Name:     col.Name,
			HasKids:  hasKids,
			ReqCount: len(col.Requests),
		})
		if hasKids && expanded[col.ID] {
			nodes = append(nodes, BuildTree(col.Children, expanded, depth+1, path)...)
		}
	}
	return nodes
}

func PathEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ── Panel ─────────────────────────────────────────────────────────────────────

type CollectionsPanel struct {
	Cursor       int
	ScrollOffset int
	Expanded     map[string]bool
	Filter       string
	SearchActive bool   // render search bar inside panel
	SearchView   string // pre-rendered search input string (with cursor)
	Focused      bool
	Width        int
	Height       int
}

func NewCollectionsPanel() CollectionsPanel {
	return CollectionsPanel{Expanded: map[string]bool{}}
}

func (p CollectionsPanel) visible(cols []storage.Collection) []TreeNode {
	if p.Filter == "" {
		return BuildTree(cols, p.Expanded, 0, nil)
	}
	return FuzzySearchCollections(cols, p.Filter)
}

func (p *CollectionsPanel) SetFilter(f string) {
	p.Filter = f
	p.Cursor = 0
	p.ScrollOffset = 0
}

// ExpandPath expands all ancestor collections so the node at path becomes
// visible in the normal (non-filtered) tree, then positions the cursor on it.
func (p *CollectionsPanel) ExpandPath(cols []storage.Collection, path []int) {
	if p.Expanded == nil {
		p.Expanded = map[string]bool{}
	}
	cur := cols
	for depth := 0; depth < len(path)-1; depth++ {
		idx := path[depth]
		if idx >= len(cur) {
			break
		}
		p.Expanded[cur[idx].ID] = true
		cur = cur[idx].Children
	}
	p.Cursor = p.CursorForPath(cols, path)
	p.clampScroll()
}

// listRows returns the number of rows available for list items.
func (p *CollectionsPanel) listRows() int {
	if p.Height == 0 {
		return 10
	}
	r := p.Height - 4 // border(2) + title(1) + margin
	if p.SearchActive {
		r -= 2 // separator + input line
	}
	if r < 1 {
		r = 1
	}
	return r
}

func (p *CollectionsPanel) clampScroll() {
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

func (p *CollectionsPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
	p.clampScroll()
}

func (p *CollectionsPanel) MoveDown(cols []storage.Collection) {
	if p.Cursor < len(p.visible(cols))-1 {
		p.Cursor++
	}
	p.clampScroll()
}

func (p *CollectionsPanel) GoTop() {
	p.Cursor = 0
	p.ScrollOffset = 0
}

func (p *CollectionsPanel) GoBottom(cols []storage.Collection) {
	n := len(p.visible(cols))
	if n > 0 {
		p.Cursor = n - 1
	}
	p.clampScroll()
}

func (p *CollectionsPanel) Toggle(cols []storage.Collection) {
	nodes := p.visible(cols)
	if p.Cursor >= len(nodes) {
		return
	}
	n := nodes[p.Cursor]
	if !n.HasKids {
		return
	}
	if p.Expanded == nil {
		p.Expanded = map[string]bool{}
	}
	if p.Expanded[n.ID] {
		delete(p.Expanded, n.ID)
	} else {
		p.Expanded[n.ID] = true
	}
	p.clampScroll()
}

func (p *CollectionsPanel) CollapseToParent(cols []storage.Collection) {
	nodes := p.visible(cols)
	if p.Cursor >= len(nodes) {
		return
	}
	n := nodes[p.Cursor]
	if n.HasKids && p.Expanded[n.ID] {
		delete(p.Expanded, n.ID)
		p.clampScroll()
		return
	}
	for i := p.Cursor - 1; i >= 0; i-- {
		if nodes[i].Depth == n.Depth-1 {
			p.Cursor = i
			p.clampScroll()
			return
		}
	}
}

func (p CollectionsPanel) SelectedPath(cols []storage.Collection) []int {
	nodes := p.visible(cols)
	if p.Cursor >= len(nodes) {
		return nil
	}
	return nodes[p.Cursor].Path
}

func (p CollectionsPanel) SelectedHasKids(cols []storage.Collection) bool {
	nodes := p.visible(cols)
	if p.Cursor >= len(nodes) {
		return false
	}
	return nodes[p.Cursor].HasKids
}

func (p CollectionsPanel) CursorForPath(cols []storage.Collection, path []int) int {
	nodes := p.visible(cols)
	for i, n := range nodes {
		if PathEqual(n.Path, path) {
			return i
		}
	}
	return 0
}

func (p CollectionsPanel) View(cols []storage.Collection) string {
	style := PanelStyle
	titleSt := TitleStyle
	if p.Focused {
		style = PanelFocusedStyle
		titleSt = TitleFocusedStyle
	}

	title := titleSt.Render("Collections")
	nodes := p.visible(cols)
	itemW := p.Width - 6
	rows := p.listRows()

	var sb strings.Builder
	sb.WriteString(title + "\n")

	if len(nodes) == 0 {
		if p.Filter != "" {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No matches") + "\n")
		} else {
			sb.WriteString(MutedStyle.Padding(0, 1).Render("No collections") + "\n")
		}
	}

	// Render only the visible window
	searching := p.Filter != ""
	end := p.ScrollOffset + rows
	if end > len(nodes) {
		end = len(nodes)
	}
	for i := p.ScrollOffset; i < end; i++ {
		n := nodes[i]

		var label, selectedLabel string
		if searching {
			// Flat fuzzy results: show full breadcrumb, no tree chrome
			reqBadge := ""
			if n.ReqCount > 0 {
				reqBadge = MutedStyle.Render(fmt.Sprintf(" (%d)", n.ReqCount))
			}
			label = n.Breadcrumb + reqBadge
			selectedLabel = n.Breadcrumb
		} else {
			indent := strings.Repeat("  ", n.Depth)
			icon := "·"
			if n.HasKids {
				if p.Expanded[n.ID] {
					icon = "▼"
				} else {
					icon = "▶"
				}
			}
			iconStr := lipgloss.NewStyle().Foreground(ColorMuted).Render(icon)
			reqBadge := ""
			if n.ReqCount > 0 {
				reqBadge = MutedStyle.Render(fmt.Sprintf(" (%d)", n.ReqCount))
			}
			label = indent + iconStr + " " + n.Name + reqBadge
			selectedLabel = indent + icon + " " + n.Name
		}

		if i == p.Cursor && p.Focused {
			sb.WriteString(SelectedItemStyle.Width(itemW).Render(selectedLabel) + "\n")
		} else if i == p.Cursor {
			sb.WriteString(lipgloss.NewStyle().Foreground(ColorSelectedText).Width(itemW).Padding(0, 1).Render(label) + "\n")
		} else {
			sb.WriteString(NormalItemStyle.Width(itemW).Render(label) + "\n")
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
