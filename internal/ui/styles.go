package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorBorder        = lipgloss.Color("#3C3C3C")
	ColorBorderFocused = lipgloss.Color("#7AA2F7")
	ColorTitle         = lipgloss.Color("#7AA2F7")
	ColorTitleFocused  = lipgloss.Color("#BB9AF7")
	ColorSelected      = lipgloss.Color("#2D4F67")
	ColorSelectedText  = lipgloss.Color("#7DCFFF")
	ColorMuted         = lipgloss.Color("#565F89")
	ColorText          = lipgloss.Color("#C0CAF5")
	ColorStatusBar     = lipgloss.Color("#1A1B26")
	ColorMethodGET     = lipgloss.Color("#9ECE6A")
	ColorMethodPOST    = lipgloss.Color("#E0AF68")
	ColorMethodPUT     = lipgloss.Color("#7AA2F7")
	ColorMethodDELETE  = lipgloss.Color("#F7768E")
	ColorMethodPATCH   = lipgloss.Color("#BB9AF7")
	ColorSuccess       = lipgloss.Color("#9ECE6A")
	ColorError         = lipgloss.Color("#F7768E")
	ColorWarning       = lipgloss.Color("#E0AF68")
	ColorModeNormal    = lipgloss.Color("#9ECE6A")
	ColorModeInsert    = lipgloss.Color("#7AA2F7")
	ColorModeCommand   = lipgloss.Color("#E0AF68")
)

var (
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	PanelFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorderFocused).
				Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorTitle).
			Bold(true).
			Padding(0, 1)

	TitleFocusedStyle = lipgloss.NewStyle().
				Foreground(ColorTitleFocused).
				Bold(true).
				Padding(0, 1)

	SelectedItemStyle = lipgloss.NewStyle().
				Background(ColorSelected).
				Foreground(ColorSelectedText).
				Bold(true).
				Padding(0, 1)

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 1)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StatusBarStyle = lipgloss.NewStyle().
			Background(ColorStatusBar).
			Foreground(ColorText).
			Padding(0, 1)

	KeyHintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	KeyStyle = lipgloss.NewStyle().
			Foreground(ColorBorderFocused).
			Bold(true)
)

func MethodStyle(method string) lipgloss.Style {
	s := lipgloss.NewStyle().Bold(true).Width(7)
	switch method {
	case "GET":
		return s.Foreground(ColorMethodGET)
	case "POST":
		return s.Foreground(ColorMethodPOST)
	case "PUT":
		return s.Foreground(ColorMethodPUT)
	case "DELETE":
		return s.Foreground(ColorMethodDELETE)
	case "PATCH":
		return s.Foreground(ColorMethodPATCH)
	default:
		return s.Foreground(ColorMuted)
	}
}
