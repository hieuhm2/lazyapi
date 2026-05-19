package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// PlaceOverlay centers popup on top of bg without hiding the background.
func PlaceOverlay(bg, popup string, termW, termH int) string {
	bgLines := strings.Split(bg, "\n")
	popupLines := strings.Split(popup, "\n")

	popupH := len(popupLines)
	popupW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popupW {
			popupW = w
		}
	}

	startX := (termW - popupW) / 2
	startY := (termH - popupH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	result := make([]string, len(bgLines))
	copy(result, bgLines)

	for i, pLine := range popupLines {
		y := startY + i
		if y < 0 || y >= len(result) {
			continue
		}
		result[y] = mergeLine(result[y], pLine, startX, termW)
	}

	return strings.Join(result, "\n")
}

// mergeLine replaces columns [x, x+width(fg)) of bgLine with fgLine.
func mergeLine(bgLine, fgLine string, x, termW int) string {
	// Left: ANSI-aware truncation to x columns
	left := lipgloss.NewStyle().MaxWidth(x).Render(bgLine)
	leftW := lipgloss.Width(left)
	if leftW < x {
		left += strings.Repeat(" ", x-leftW)
	}

	// Right: skip first x+fgW columns (strip ANSI — acceptable for edge content)
	fgW := lipgloss.Width(fgLine)
	right := columnSkip(stripANSI(bgLine), x+fgW)

	return left + fgLine + right
}

// columnSkip removes the first n visual columns from plain (no ANSI) text.
func columnSkip(plain string, n int) string {
	col := 0
	for i := 0; i < len(plain); {
		if col >= n {
			return plain[i:]
		}
		r, size := utf8.DecodeRuneInString(plain[i:])
		col += runewidth.RuneWidth(r)
		i += size
	}
	return ""
}

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) {
				c := s[i]
				i++
				if c >= 0x40 && c <= 0x7E {
					break
				}
			}
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}
