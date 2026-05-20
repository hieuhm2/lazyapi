package ui

import (
	"sort"
	"strings"

	"github.com/hieuhm2/lazyapi/internal/storage"
)

// fuzzyScore returns a score ≥ 0 if all pattern chars appear in order in text,
// or -1 if they don't. Consecutive matches, start-of-string, and word-boundary
// hits score higher.
func fuzzyScore(pattern, text string) int {
	if pattern == "" {
		return 0
	}
	p := strings.ToLower(pattern)
	t := strings.ToLower(text)

	pi, score, consecutive, lastMatch := 0, 0, 0, -1
	for ti := 0; ti < len(t) && pi < len(p); ti++ {
		if t[ti] != p[pi] {
			continue
		}
		bonus := 1
		if ti == 0 {
			bonus += 9
		}
		if lastMatch == ti-1 {
			consecutive++
			bonus += 4 * consecutive
		} else {
			consecutive = 0
			if ti > 0 {
				prev := t[ti-1]
				if prev == ' ' || prev == '/' || prev == '_' || prev == '-' || prev == '.' {
					bonus += 2
				}
			}
		}
		lastMatch = ti
		pi++
		score += bonus
	}

	if pi != len(p) {
		return -1
	}
	return score
}

func scoreBest(scores ...int) int {
	b := -1
	for _, s := range scores {
		if s > b {
			b = s
		}
	}
	return b
}

// ── Global request search ─────────────────────────────────────────────────────

// GlobalRequest is a request result from a recursive search across all collections.
type GlobalRequest struct {
	Req           storage.Request
	ColPath       []int
	ColBreadcrumb string
	Score         int
}

// FuzzySearchRequests searches all requests in the full collection tree and
// returns results sorted by descending match score.
func FuzzySearchRequests(cols []storage.Collection, pattern string) []GlobalRequest {
	if pattern == "" {
		return nil
	}
	var out []GlobalRequest
	collectRequests(cols, pattern, nil, "", &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func collectRequests(cols []storage.Collection, pattern string, path []int, breadcrumb string, out *[]GlobalRequest) {
	for i, col := range cols {
		p := append(append([]int(nil), path...), i)
		bc := col.Name
		if breadcrumb != "" {
			bc = breadcrumb + " › " + col.Name
		}
		for _, req := range col.Requests {
			s := scoreBest(
				fuzzyScore(pattern, req.Name),
				fuzzyScore(pattern, req.Method),
				fuzzyScore(pattern, req.URL),
			)
			if s >= 0 {
				*out = append(*out, GlobalRequest{Req: req, ColPath: p, ColBreadcrumb: bc, Score: s})
			}
		}
		collectRequests(col.Children, pattern, p, bc, out)
	}
}

// ── Global collection search ──────────────────────────────────────────────────

type fuzzyColResult struct {
	node  TreeNode
	score int
}

// FuzzySearchCollections returns all matching collections from the full tree
// (ignoring expand state), sorted by descending score. Each node's Breadcrumb
// field holds the full ancestor path for display.
func FuzzySearchCollections(cols []storage.Collection, pattern string) []TreeNode {
	if pattern == "" {
		return nil
	}
	var candidates []fuzzyColResult
	collectCollections(cols, pattern, nil, "", 0, &candidates)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	nodes := make([]TreeNode, len(candidates))
	for i, c := range candidates {
		nodes[i] = c.node
	}
	return nodes
}

func collectCollections(cols []storage.Collection, pattern string, path []int, breadcrumb string, depth int, out *[]fuzzyColResult) {
	for i, col := range cols {
		p := append(append([]int(nil), path...), i)
		bc := col.Name
		if breadcrumb != "" {
			bc = breadcrumb + " › " + col.Name
		}
		if score := fuzzyScore(pattern, col.Name); score >= 0 {
			*out = append(*out, fuzzyColResult{
				score: score,
				node: TreeNode{
					Path:       p,
					Depth:      depth,
					ID:         col.ID,
					Name:       col.Name,
					Breadcrumb: bc,
					HasKids:    len(col.Children) > 0,
					ReqCount:   len(col.Requests),
				},
			})
		}
		collectCollections(col.Children, pattern, p, bc, depth+1, out)
	}
}
