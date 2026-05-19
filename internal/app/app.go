package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	httpclient "github.com/hieuhm2/lazyapi/internal/http"
	"github.com/hieuhm2/lazyapi/internal/storage"
	"github.com/hieuhm2/lazyapi/internal/ui"
)

type ResponseMsg struct {
	Response storage.Response
}

type App struct {
	mode        Mode
	focused     Panel
	ggPressed   bool // tracks first 'g' for gg motion
	width       int
	height      int
	Collections ui.CollectionsPanel
	Requests    ui.RequestsPanel
	Editor      ui.EditorPanel
	Response    ui.ResponsePanel
	keys        KeyMap
	statusMsg   string
}

func New() App {
	a := App{
		mode:        ModeNormal,
		focused:     PanelCollections,
		Collections: ui.NewCollectionsPanel(),
		Requests:    ui.NewRequestsPanel(),
		Editor:      ui.NewEditorPanel(),
		Response:    ui.NewResponsePanel(),
		keys:        DefaultKeyMap,
	}
	// sync initial state
	if col := a.Collections.Selected(); col != nil {
		a.Requests.SetRequests(col.Requests)
	}
	if req := a.Requests.Selected(); req != nil {
		a.Editor.SetRequest(req)
	}
	a.syncFocus()
	return a
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.recalcSizes()
		return a, nil

	case ResponseMsg:
		a.Response.SetResponse(&msg.Response)
		a.statusMsg = fmt.Sprintf("Response received: %s", msg.Response.Status)
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global: always allow ctrl+c
	if msg.String() == "ctrl+c" {
		return a, tea.Quit
	}

	switch a.mode {
	case ModeNormal:
		return a.handleNormal(msg)
	case ModeInsert:
		return a.handleInsert(msg)
	case ModeCommand:
		return a.handleCommand(msg)
	}
	return a, nil
}

func (a App) handleNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Panel jump
	switch k {
	case "1":
		a.focused = PanelCollections
		a.syncFocus()
		return a, nil
	case "2":
		a.focused = PanelRequests
		a.syncFocus()
		return a, nil
	case "3":
		a.focused = PanelEditor
		a.syncFocus()
		return a, nil
	case "4":
		a.focused = PanelResponse
		a.syncFocus()
		return a, nil
	case "tab":
		a.focused = Panel((int(a.focused) + 1) % int(PanelCount))
		a.syncFocus()
		return a, nil
	case "shift+tab":
		a.focused = Panel((int(a.focused) - 1 + int(PanelCount)) % int(PanelCount))
		a.syncFocus()
		return a, nil
	case "q":
		return a, tea.Quit
	case "?":
		a.statusMsg = "Tab/1-4: panels  hjkl: navigate  r/enter: execute  n: new  d: delete  i: insert  ?: help"
		return a, nil
	}

	// Navigation within focused panel
	switch k {
	case "j", "down":
		a.moveDown()
	case "k", "up":
		a.moveUp()
	case "G":
		a.goBottom()
		a.ggPressed = false
	case "g":
		if a.ggPressed {
			a.goTop()
			a.ggPressed = false
		} else {
			a.ggPressed = true
		}
		return a, nil
	case "ctrl+d":
		a.halfDown()
	case "ctrl+u":
		a.halfUp()
	case "l", "right":
		if a.focused == PanelCollections {
			a.focused = PanelRequests
			a.syncFocus()
		} else if a.focused == PanelRequests {
			a.focused = PanelEditor
			a.syncFocus()
		}
	case "h", "left":
		if a.focused == PanelEditor {
			a.focused = PanelRequests
			a.syncFocus()
		} else if a.focused == PanelRequests {
			a.focused = PanelCollections
			a.syncFocus()
		}
	case "enter", "r":
		return a.executeRequest()
	case "i":
		if a.focused == PanelEditor {
			a.mode = ModeInsert
			a.Editor.InsertMode = true
			a.Editor.URLInput.Focus()
			a.statusMsg = ""
		}
	case "\t": // handled above but guard
	}

	a.ggPressed = false
	return a, nil
}

func (a App) handleInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "esc" {
		a.mode = ModeNormal
		a.Editor.InsertMode = false
		a.Editor.URLInput.Blur()
		a.Editor.BodyInput.Blur()
		return a, nil
	}

	var cmd tea.Cmd
	a.Editor.URLInput, cmd = a.Editor.URLInput.Update(msg)
	if a.Editor.Request != nil {
		a.Editor.Request.URL = a.Editor.URLInput.Value()
	}
	return a, cmd
}

func (a App) handleCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.mode = ModeNormal
	}
	return a, nil
}

func (a *App) moveUp() {
	switch a.focused {
	case PanelCollections:
		a.Collections.MoveUp()
		a.syncCollectionSelection()
	case PanelRequests:
		a.Requests.MoveUp()
		a.syncRequestSelection()
	case PanelResponse:
		a.Response.Viewport.LineUp(1)
	}
}

func (a *App) moveDown() {
	switch a.focused {
	case PanelCollections:
		a.Collections.MoveDown()
		a.syncCollectionSelection()
	case PanelRequests:
		a.Requests.MoveDown()
		a.syncRequestSelection()
	case PanelResponse:
		a.Response.Viewport.LineDown(1)
	}
}

func (a *App) goTop() {
	switch a.focused {
	case PanelCollections:
		a.Collections.GoTop()
		a.syncCollectionSelection()
	case PanelRequests:
		a.Requests.GoTop()
		a.syncRequestSelection()
	case PanelResponse:
		a.Response.Viewport.GotoTop()
	}
}

func (a *App) goBottom() {
	switch a.focused {
	case PanelCollections:
		a.Collections.GoBottom()
		a.syncCollectionSelection()
	case PanelRequests:
		a.Requests.GoBottom()
		a.syncRequestSelection()
	case PanelResponse:
		a.Response.Viewport.GotoBottom()
	}
}

func (a *App) halfDown() {
	switch a.focused {
	case PanelResponse:
		a.Response.Viewport.HalfViewDown()
	case PanelRequests:
		for i := 0; i < 5; i++ {
			a.Requests.MoveDown()
		}
		a.syncRequestSelection()
	case PanelCollections:
		for i := 0; i < 5; i++ {
			a.Collections.MoveDown()
		}
		a.syncCollectionSelection()
	}
}

func (a *App) halfUp() {
	switch a.focused {
	case PanelResponse:
		a.Response.Viewport.HalfViewUp()
	case PanelRequests:
		for i := 0; i < 5; i++ {
			a.Requests.MoveUp()
		}
		a.syncRequestSelection()
	case PanelCollections:
		for i := 0; i < 5; i++ {
			a.Collections.MoveUp()
		}
		a.syncCollectionSelection()
	}
}

func (a *App) syncCollectionSelection() {
	if col := a.Collections.Selected(); col != nil {
		a.Requests.SetRequests(col.Requests)
		if req := a.Requests.Selected(); req != nil {
			a.Editor.SetRequest(req)
		}
	}
}

func (a *App) syncRequestSelection() {
	if req := a.Requests.Selected(); req != nil {
		a.Editor.SetRequest(req)
	}
}

func (a *App) syncFocus() {
	a.Collections.Focused = a.focused == PanelCollections
	a.Requests.Focused = a.focused == PanelRequests
	a.Editor.Focused = a.focused == PanelEditor
	a.Response.Focused = a.focused == PanelResponse
}

func (a App) executeRequest() (tea.Model, tea.Cmd) {
	req := a.Requests.Selected()
	if req == nil {
		a.statusMsg = "No request selected"
		return a, nil
	}
	a.Response.SetLoading()
	a.statusMsg = fmt.Sprintf("Sending %s %s...", req.Method, req.URL)
	snapshot := *req
	return a, func() tea.Msg {
		resp := httpclient.Execute(snapshot)
		return ResponseMsg{Response: resp}
	}
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Layout dimensions
	topHeight := a.height - 8  // row 1: 3 panels
	botHeight := 8             // row 2: response
	colW := a.width / 5        // collections ~20%
	reqW := a.width / 5        // requests ~20%
	editorW := a.width - colW - reqW // editor fills rest

	a.Collections.Width = colW
	a.Collections.Height = topHeight
	a.Requests.Width = reqW
	a.Requests.Height = topHeight
	a.Editor.Width = editorW
	a.Editor.Height = topHeight
	a.Response.Width = a.width
	a.Response.Height = botHeight

	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		a.Collections.View(),
		a.Requests.View(),
		a.Editor.View(),
	)

	botRow := a.Response.View()
	statusBar := a.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, topRow, botRow, statusBar)
}

func (a *App) recalcSizes() {
	topHeight := a.height - 8
	botHeight := 8
	colW := a.width / 5
	reqW := a.width / 5
	editorW := a.width - colW - reqW

	a.Collections.Width = colW
	a.Collections.Height = topHeight
	a.Requests.Width = reqW
	a.Requests.Height = topHeight
	a.Editor.Width = editorW
	a.Editor.Height = topHeight
	a.Response.Width = a.width
	a.Response.Height = botHeight
}

func (a App) renderStatusBar() string {
	mode := a.mode.String()
	modeColor := ui.ColorModeNormal
	if a.mode == ModeInsert {
		modeColor = ui.ColorModeInsert
	} else if a.mode == ModeCommand {
		modeColor = ui.ColorModeCommand
	}

	modeLabel := lipgloss.NewStyle().
		Background(modeColor).
		Foreground(lipgloss.Color("#1A1B26")).
		Bold(true).
		Padding(0, 1).
		Render(mode)

	panel := ui.MutedStyle.Render(fmt.Sprintf(" %s ", a.focused.String()))

	hints := buildHints(a.mode, a.focused)
	hintStr := ui.KeyHintStyle.Render(hints)

	msg := ""
	if a.statusMsg != "" {
		msg = lipgloss.NewStyle().Foreground(ui.ColorWarning).Render("  " + a.statusMsg)
	}

	left := modeLabel + panel + msg
	right := hintStr

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return ui.StatusBarStyle.Width(a.width).Render(
		left + strings.Repeat(" ", gap) + right,
	)
}

func buildHints(mode Mode, panel Panel) string {
	if mode == ModeInsert {
		return "<Esc> normal  <Tab> next tab"
	}
	base := "<Tab> panel  <hjkl> nav  <r> run  <n> new  <d> del  <q> quit  <?> help"
	if panel == PanelEditor {
		base = "<i> insert  " + base
	}
	return base
}
