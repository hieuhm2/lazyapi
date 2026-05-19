package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

// ── Tree node ─────────────────────────────────────────────────────────────────

// TreeNode is a flattened entry in the visible tree (one per rendered row).
type TreeNode struct {
	Path     []int  // path of indices to reach this collection
	Depth    int
	ID       string
	Name     string
	HasKids  bool
	ReqCount int
}

// BuildTree returns the flat list of nodes currently visible (respecting expanded).
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

// PathEqual compares two paths.
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
	Cursor   int              // index into the visible flat tree
	Expanded map[string]bool  // collection ID → expanded
	Filter   string
	Focused  bool
	Width    int
	Height   int
}

func NewCollectionsPanel() CollectionsPanel {
	return CollectionsPanel{Expanded: map[string]bool{}}
}

func (p CollectionsPanel) visible(cols []storage.Collection) []TreeNode {
	all := BuildTree(cols, p.Expanded, 0, nil)
	if p.Filter == "" {
		return all
	}
	f := strings.ToLower(p.Filter)
	var out []TreeNode
	for _, n := range all {
		if strings.Contains(strings.ToLower(n.Name), f) {
			out = append(out, n)
		}
	}
	return out
}

func (p *CollectionsPanel) SetFilter(f string) {
	p.Filter = f
	p.Cursor = 0
}

func (p *CollectionsPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

func (p *CollectionsPanel) MoveDown(cols []storage.Collection) {
	if p.Cursor < len(p.visible(cols))-1 {
		p.Cursor++
	}
}

func (p *CollectionsPanel) GoTop() { p.Cursor = 0 }

func (p *CollectionsPanel) GoBottom(cols []storage.Collection) {
	n := len(p.visible(cols))
	if n > 0 {
		p.Cursor = n - 1
	}
}

// Toggle expands or collapses the node under the cursor.
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
}

// CollapseToParent collapses the node if expanded, or moves cursor to parent.
func (p *CollectionsPanel) CollapseToParent(cols []storage.Collection) {
	nodes := p.visible(cols)
	if p.Cursor >= len(nodes) {
		return
	}
	n := nodes[p.Cursor]
	if n.HasKids && p.Expanded[n.ID] {
		delete(p.Expanded, n.ID)
		return
	}
	// Find parent: last node with depth = n.Depth-1, at cursor position < current
	for i := p.Cursor - 1; i >= 0; i-- {
		if nodes[i].Depth == n.Depth-1 {
			p.Cursor = i
			return
		}
	}
}

// SelectedPath returns the []int path of the currently selected node.
func (p CollectionsPanel) SelectedPath(cols []storage.Collection) []int {
	nodes := p.visible(cols)
	if p.Cursor >= len(nodes) {
		return nil
	}
	return nodes[p.Cursor].Path
}

// SelectedHasKids returns true if the selected node has children.
func (p CollectionsPanel) SelectedHasKids(cols []storage.Collection) bool {
	nodes := p.visible(cols)
	if p.Cursor >= len(nodes) {
		return false
	}
	return nodes[p.Cursor].HasKids
}

// CursorForPath returns the cursor index for a given path (or 0 if not found).
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
	if p.Filter != "" {
		title = titleSt.Render(fmt.Sprintf("Collections /%s", p.Filter))
	}

	nodes := p.visible(cols)
	itemW := p.Width - 6

	var sb strings.Builder
	sb.WriteString(title + "\n")

	if len(nodes) == 0 {
		sb.WriteString(MutedStyle.Padding(0, 1).Render("No collections") + "\n")
	}

	for i, n := range nodes {
		indent := strings.Repeat("  ", n.Depth)

		// Expand icon
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
		label := indent + iconStr + " " + n.Name + reqBadge

		if i == p.Cursor && p.Focused {
			sb.WriteString(SelectedItemStyle.Width(itemW).Render(indent+icon+" "+n.Name) + "\n")
		} else if i == p.Cursor {
			sb.WriteString(lipgloss.NewStyle().Foreground(ColorSelectedText).Width(itemW).Padding(0, 1).Render(label) + "\n")
		} else {
			sb.WriteString(NormalItemStyle.Width(itemW).Render(label) + "\n")
		}
	}

	return style.Width(p.Width - 2).Height(p.Height - 2).Render(sb.String())
}
