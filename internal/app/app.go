package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	httpclient "github.com/hieuhm2/lazyapi/internal/http"
	"github.com/hieuhm2/lazyapi/internal/storage"
	"github.com/hieuhm2/lazyapi/internal/ui"
)

// ── messages ────────────────────────────────────────────────────────────────

type responseMsg struct{ resp storage.Response }
type savedMsg struct{ err error }
type editorDoneMsg struct {
	content string
	err     error
}

// ── App ─────────────────────────────────────────────────────────────────────

type App struct {
	// Core state
	mode    Mode
	state   AppState
	focused Panel

	// Vim motion helpers
	ggPressed bool

	// Terminal size
	width, height int

	// Data (source of truth lives here)
	collections []storage.Collection
	colIdx      int // selected collection index
	reqIdx      int // selected request index
	envFile     storage.EnvFile
	activeEnv   *storage.Environment

	// Panels (view state only)
	colPanel  ui.CollectionsPanel
	reqPanel  ui.RequestsPanel
	editor    ui.EditorPanel
	response  ui.ResponsePanel

	// Overlay inputs
	overlayInput textinput.Model
	overlayTitle string
	deleteTarget string

	// Search
	searchInput textinput.Model

	// Header editing (two-step input)
	pendingHeaderKey string

	// Persistence
	store *storage.Store

	statusMsg string
}

func New() App {
	overlayIn := textinput.New()
	overlayIn.CharLimit = 128

	searchIn := textinput.New()
	searchIn.Placeholder = "search..."
	searchIn.CharLimit = 64

	a := App{
		mode:         ModeNormal,
		state:        StateDefault,
		focused:      PanelCollections,
		editor:       ui.NewEditorPanel(),
		response:     ui.NewResponsePanel(),
		overlayInput: overlayIn,
		searchInput:  searchIn,
	}

	// Load from disk (fall back to defaults on error)
	store, err := storage.NewStore()
	if err == nil {
		a.store = store
		cols, err := store.LoadCollections()
		if err == nil {
			a.collections = cols
		}
		ef, err := store.LoadEnvFile()
		if err == nil {
			a.envFile = ef
		}
	}
	if len(a.collections) == 0 {
		a.collections = storage.DefaultCollections()
	}
	if len(a.envFile.Envs) == 0 {
		a.envFile = storage.DefaultEnvFile()
	}
	a.syncActiveEnv()
	a.syncPanels()
	a.syncFocus()
	return a
}

func (a App) Init() tea.Cmd { return nil }

// ── Update ───────────────────────────────────────────────────────────────────

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case responseMsg:
		r := msg.resp
		a.response.SetResponse(&r)
		if r.Error != "" {
			a.statusMsg = "Error: " + r.Error
		} else {
			a.statusMsg = fmt.Sprintf("%s  ·  %dms  ·  %s",
				r.Status,
				r.DurationMs,
				formatSize(r.SizeBytes),
			)
		}
		return a, nil

	case editorDoneMsg:
		if msg.err == nil && msg.content != "" {
			if a.colIdx < len(a.collections) {
				reqs := a.collections[a.colIdx].Requests
				if a.reqIdx < len(reqs) {
					a.collections[a.colIdx].Requests[a.reqIdx].Body = msg.content
					a.editor.SetRequest(&a.collections[a.colIdx].Requests[a.reqIdx])
					return a, a.save()
				}
			}
		}
		return a, nil

	case savedMsg:
		if msg.err != nil {
			a.statusMsg = "Save error: " + msg.err.Error()
		}
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return a, tea.Quit
	}
	switch a.state {
	case StateHelp:
		a.state = StateDefault
		return a, nil
	case StateNewCollection, StateNewRequest, StateRename,
		StateNewHeaderKey, StateNewHeaderValue, StateEditHeaderValue:
		return a.handleOverlay(msg)
	case StateDeleteConfirm:
		return a.handleDeleteConfirm(msg)
	case StateSearching:
		return a.handleSearch(msg)
	default:
		if a.mode == ModeInsert {
			return a.handleInsert(msg)
		}
		return a.handleNormal(msg)
	}
}

// ── Normal mode ──────────────────────────────────────────────────────────────

func (a App) handleNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Panel jumps
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
		a.state = StateHelp
		return a, nil
	}

	// Actions (with per-tab overrides for the editor panel)
	switch k {
	case "r", "enter":
		return a.executeRequest()
	case "n":
		if a.focused == PanelEditor && a.editor.ActiveTab == ui.EditorTabHeaders {
			return a.startNewHeader()
		}
		return a.startNew()
	case "d":
		if a.focused == PanelEditor && a.editor.ActiveTab == ui.EditorTabHeaders {
			return a.deleteSelectedHeader()
		}
		return a.startDelete()
	case "e":
		if a.focused == PanelEditor && a.editor.ActiveTab == ui.EditorTabHeaders {
			return a.startEditHeader()
		}
		if a.focused == PanelEditor && a.editor.ActiveTab == ui.EditorTabBody {
			return a.openExternalEditor()
		}
		return a.startRename()
	case "m":
		if a.focused == PanelEditor {
			a.editor.CycleMethod()
			if a.colIdx < len(a.collections) {
				reqs := a.collections[a.colIdx].Requests
				if a.reqIdx < len(reqs) {
					a.collections[a.colIdx].Requests[a.reqIdx].Method = a.editor.Request.Method
					return a, a.save()
				}
			}
		}
		return a, nil
	case "t":
		if a.focused == PanelEditor {
			a.editor.NextTab()
		} else if a.focused == PanelResponse {
			a.response.NextTab()
		}
		return a, nil
	case "T":
		if a.focused == PanelEditor {
			a.editor.PrevTab()
		}
		return a, nil
	case "i":
		if a.focused == PanelEditor {
			a.mode = ModeInsert
			a.editor.InsertMode = true
			a.editor.URLInput.Focus()
		}
		return a, nil
	case "E":
		return a.cycleEnv()
	case "/":
		if a.focused == PanelCollections || a.focused == PanelRequests {
			a.state = StateSearching
			a.searchInput.SetValue("")
			a.searchInput.Focus()
			a.statusMsg = ""
		}
		return a, nil
	}

	// Navigation
	switch k {
	case "j", "down":
		if a.focused == PanelEditor && a.editor.ActiveTab == ui.EditorTabHeaders {
			a.editor.MoveHeaderDown()
		} else {
			a.moveDown()
		}
	case "k", "up":
		if a.focused == PanelEditor && a.editor.ActiveTab == ui.EditorTabHeaders {
			a.editor.MoveHeaderUp()
		} else {
			a.moveUp()
		}
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
		switch a.focused {
		case PanelCollections:
			a.focused = PanelRequests
		case PanelRequests:
			a.focused = PanelEditor
		case PanelEditor:
			a.focused = PanelResponse
		}
		a.syncFocus()
	case "h", "left":
		switch a.focused {
		case PanelResponse:
			a.focused = PanelEditor
		case PanelEditor:
			a.focused = PanelRequests
		case PanelRequests:
			a.focused = PanelCollections
		}
		a.syncFocus()
	}

	a.ggPressed = false
	return a, nil
}

// ── Insert mode ───────────────────────────────────────────────────────────────

func (a App) handleInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.mode = ModeNormal
		a.editor.InsertMode = false
		a.editor.URLInput.Blur()
		// Write URL back to source
		if a.colIdx < len(a.collections) {
			reqs := a.collections[a.colIdx].Requests
			if a.reqIdx < len(reqs) {
				a.collections[a.colIdx].Requests[a.reqIdx].URL = a.editor.URLInput.Value()
				return a, a.save()
			}
		}
		return a, nil
	}

	var cmd tea.Cmd
	a.editor.URLInput, cmd = a.editor.URLInput.Update(msg)
	return a, cmd
}

// ── Overlay (new/rename/header editing) ─────────────────────────────────────

func (a App) handleOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.state = StateDefault
		a.overlayInput.Blur()
		a.pendingHeaderKey = ""
		return a, nil
	}

	if msg.String() != "enter" {
		var cmd tea.Cmd
		a.overlayInput, cmd = a.overlayInput.Update(msg)
		return a, cmd
	}

	// Enter pressed — dispatch per state
	switch a.state {
	case StateNewCollection:
		val := strings.TrimSpace(a.overlayInput.Value())
		if val != "" {
			a.collections = append(a.collections, storage.Collection{ID: storage.NewID(), Name: val})
			a.colIdx = len(a.collections) - 1
			a.reqIdx = 0
		}
		a.state = StateDefault
		a.overlayInput.Blur()
		a.syncPanels()
		return a, a.save()

	case StateNewRequest:
		val := strings.TrimSpace(a.overlayInput.Value())
		if val != "" && a.colIdx < len(a.collections) {
			newReq := storage.Request{ID: storage.NewID(), Name: val, Method: "GET"}
			a.collections[a.colIdx].Requests = append(a.collections[a.colIdx].Requests, newReq)
			a.reqIdx = len(a.collections[a.colIdx].Requests) - 1
		}
		a.state = StateDefault
		a.overlayInput.Blur()
		a.syncPanels()
		return a, a.save()

	case StateRename:
		val := strings.TrimSpace(a.overlayInput.Value())
		if val != "" {
			switch a.focused {
			case PanelCollections:
				if a.colIdx < len(a.collections) {
					a.collections[a.colIdx].Name = val
				}
			case PanelRequests:
				if a.colIdx < len(a.collections) {
					reqs := a.collections[a.colIdx].Requests
					if a.reqIdx < len(reqs) {
						a.collections[a.colIdx].Requests[a.reqIdx].Name = val
					}
				}
			}
		}
		a.state = StateDefault
		a.overlayInput.Blur()
		a.syncPanels()
		return a, a.save()

	case StateNewHeaderKey:
		key := strings.TrimSpace(a.overlayInput.Value())
		if key == "" {
			a.state = StateDefault
			a.overlayInput.Blur()
			return a, nil
		}
		// Step 2: ask for the value
		a.pendingHeaderKey = key
		a.state = StateNewHeaderValue
		a.overlayTitle = fmt.Sprintf("Value for '%s'", key)
		a.overlayInput.Placeholder = "header value"
		a.overlayInput.SetValue("")
		return a, nil // stay in overlay

	case StateNewHeaderValue:
		if a.colIdx < len(a.collections) {
			reqs := a.collections[a.colIdx].Requests
			if a.reqIdx < len(reqs) {
				if a.collections[a.colIdx].Requests[a.reqIdx].Headers == nil {
					a.collections[a.colIdx].Requests[a.reqIdx].Headers = make(map[string]string)
				}
				a.collections[a.colIdx].Requests[a.reqIdx].Headers[a.pendingHeaderKey] = a.overlayInput.Value()
				a.editor.SetRequest(&a.collections[a.colIdx].Requests[a.reqIdx])
			}
		}
		a.pendingHeaderKey = ""
		a.state = StateDefault
		a.overlayInput.Blur()
		return a, a.save()

	case StateEditHeaderValue:
		hKey := a.editor.SelectedHeaderKey()
		if hKey != "" && a.colIdx < len(a.collections) {
			reqs := a.collections[a.colIdx].Requests
			if a.reqIdx < len(reqs) {
				a.collections[a.colIdx].Requests[a.reqIdx].Headers[hKey] = a.overlayInput.Value()
				a.editor.SetRequest(&a.collections[a.colIdx].Requests[a.reqIdx])
			}
		}
		a.state = StateDefault
		a.overlayInput.Blur()
		return a, a.save()
	}

	return a, nil
}

// ── Delete confirm ────────────────────────────────────────────────────────────

func (a App) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		switch a.focused {
		case PanelCollections:
			if a.colIdx < len(a.collections) {
				a.collections = append(a.collections[:a.colIdx], a.collections[a.colIdx+1:]...)
				if a.colIdx > 0 {
					a.colIdx--
				}
				a.reqIdx = 0
			}
		case PanelRequests:
			if a.colIdx < len(a.collections) {
				reqs := a.collections[a.colIdx].Requests
				if a.reqIdx < len(reqs) {
					a.collections[a.colIdx].Requests = append(reqs[:a.reqIdx], reqs[a.reqIdx+1:]...)
					if a.reqIdx > 0 {
						a.reqIdx--
					}
				}
			}
		}
		a.state = StateDefault
		a.syncPanels()
		return a, a.save()
	case "n", "N", "esc":
		a.state = StateDefault
		return a, nil
	}
	return a, nil
}

// ── Search ────────────────────────────────────────────────────────────────────

func (a App) handleSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		a.state = StateDefault
		filter := a.searchInput.Value()
		a.searchInput.Blur()
		if msg.String() == "esc" {
			filter = ""
		}
		if a.focused == PanelCollections {
			a.colPanel.SetFilter(filter)
			// sync colIdx to filtered cursor
			idx := a.colPanel.SelectedIdx(a.collections)
			if idx >= 0 {
				a.colIdx = idx
			}
		} else {
			a.reqPanel.SetFilter(filter)
			if a.colIdx < len(a.collections) {
				idx := a.reqPanel.SelectedIdx(a.collections[a.colIdx].Requests)
				if idx >= 0 {
					a.reqIdx = idx
				}
			}
		}
		a.syncPanels()
		return a, nil
	}
	var cmd tea.Cmd
	a.searchInput, cmd = a.searchInput.Update(msg)
	return a, cmd
}

// ── Navigation helpers ────────────────────────────────────────────────────────

func (a *App) moveDown() {
	switch a.focused {
	case PanelCollections:
		a.colPanel.MoveDown(a.collections)
		idx := a.colPanel.SelectedIdx(a.collections)
		if idx >= 0 {
			a.colIdx = idx
		}
		a.reqIdx = 0
		a.reqPanel.Cursor = 0
		a.syncEditor()
	case PanelRequests:
		if a.colIdx < len(a.collections) {
			reqs := a.collections[a.colIdx].Requests
			a.reqPanel.MoveDown(reqs)
			idx := a.reqPanel.SelectedIdx(reqs)
			if idx >= 0 {
				a.reqIdx = idx
			}
			a.syncEditor()
		}
	case PanelResponse:
		a.response.Viewport.LineDown(1)
	}
}

func (a *App) moveUp() {
	switch a.focused {
	case PanelCollections:
		a.colPanel.MoveUp()
		idx := a.colPanel.SelectedIdx(a.collections)
		if idx >= 0 {
			a.colIdx = idx
		}
		a.reqIdx = 0
		a.reqPanel.Cursor = 0
		a.syncEditor()
	case PanelRequests:
		a.reqPanel.MoveUp()
		if a.colIdx < len(a.collections) {
			idx := a.reqPanel.SelectedIdx(a.collections[a.colIdx].Requests)
			if idx >= 0 {
				a.reqIdx = idx
			}
			a.syncEditor()
		}
	case PanelResponse:
		a.response.Viewport.LineUp(1)
	}
}

func (a *App) goTop() {
	switch a.focused {
	case PanelCollections:
		a.colPanel.GoTop()
		a.colIdx = 0
		a.reqIdx = 0
		a.reqPanel.Cursor = 0
		a.syncEditor()
	case PanelRequests:
		a.reqPanel.GoTop()
		a.reqIdx = 0
		a.syncEditor()
	case PanelResponse:
		a.response.Viewport.GotoTop()
	}
}

func (a *App) goBottom() {
	switch a.focused {
	case PanelCollections:
		a.colPanel.GoBottom(a.collections)
		idx := a.colPanel.SelectedIdx(a.collections)
		if idx >= 0 {
			a.colIdx = idx
		}
		a.reqIdx = 0
		a.reqPanel.Cursor = 0
		a.syncEditor()
	case PanelRequests:
		if a.colIdx < len(a.collections) {
			reqs := a.collections[a.colIdx].Requests
			a.reqPanel.GoBottom(reqs)
			idx := a.reqPanel.SelectedIdx(reqs)
			if idx >= 0 {
				a.reqIdx = idx
			}
			a.syncEditor()
		}
	case PanelResponse:
		a.response.Viewport.GotoBottom()
	}
}

func (a *App) halfDown() {
	for i := 0; i < 5; i++ {
		a.moveDown()
	}
}

func (a *App) halfUp() {
	for i := 0; i < 5; i++ {
		a.moveUp()
	}
}

// ── CRUD helpers ──────────────────────────────────────────────────────────────

func (a App) startNew() (tea.Model, tea.Cmd) {
	switch a.focused {
	case PanelCollections:
		a.state = StateNewCollection
		a.overlayTitle = "New Collection"
		a.overlayInput.Placeholder = "collection name"
	case PanelRequests:
		if len(a.collections) == 0 {
			a.statusMsg = "Create a collection first"
			return a, nil
		}
		a.state = StateNewRequest
		a.overlayTitle = "New Request"
		a.overlayInput.Placeholder = "request name"
	default:
		return a, nil
	}
	a.overlayInput.SetValue("")
	a.overlayInput.Focus()
	return a, nil
}

func (a App) startDelete() (tea.Model, tea.Cmd) {
	switch a.focused {
	case PanelCollections:
		if a.colIdx < len(a.collections) {
			a.deleteTarget = a.collections[a.colIdx].Name
			a.state = StateDeleteConfirm
		}
	case PanelRequests:
		if a.colIdx < len(a.collections) {
			reqs := a.collections[a.colIdx].Requests
			if a.reqIdx < len(reqs) {
				a.deleteTarget = reqs[a.reqIdx].Name
				a.state = StateDeleteConfirm
			}
		}
	}
	return a, nil
}

func (a App) startRename() (tea.Model, tea.Cmd) {
	var current string
	switch a.focused {
	case PanelCollections:
		if a.colIdx < len(a.collections) {
			current = a.collections[a.colIdx].Name
		}
	case PanelRequests:
		if a.colIdx < len(a.collections) {
			reqs := a.collections[a.colIdx].Requests
			if a.reqIdx < len(reqs) {
				current = reqs[a.reqIdx].Name
			}
		}
	default:
		return a, nil
	}
	a.state = StateRename
	a.overlayTitle = "Rename"
	a.overlayInput.Placeholder = "new name"
	a.overlayInput.SetValue(current)
	a.overlayInput.Focus()
	return a, nil
}

func (a App) startNewHeader() (tea.Model, tea.Cmd) {
	a.state = StateNewHeaderKey
	a.overlayTitle = "Header Key"
	a.overlayInput.Placeholder = "e.g. Authorization"
	a.overlayInput.SetValue("")
	a.overlayInput.Focus()
	return a, nil
}

func (a App) deleteSelectedHeader() (tea.Model, tea.Cmd) {
	key := a.editor.SelectedHeaderKey()
	if key == "" {
		return a, nil
	}
	if a.colIdx < len(a.collections) {
		reqs := a.collections[a.colIdx].Requests
		if a.reqIdx < len(reqs) {
			delete(a.collections[a.colIdx].Requests[a.reqIdx].Headers, key)
			a.editor.SetRequest(&a.collections[a.colIdx].Requests[a.reqIdx])
			if a.editor.HeaderCursor > 0 {
				a.editor.HeaderCursor--
			}
		}
	}
	return a, a.save()
}

func (a App) startEditHeader() (tea.Model, tea.Cmd) {
	key := a.editor.SelectedHeaderKey()
	if key == "" {
		return a, nil
	}
	currentVal := ""
	if a.colIdx < len(a.collections) {
		reqs := a.collections[a.colIdx].Requests
		if a.reqIdx < len(reqs) {
			currentVal = reqs[a.reqIdx].Headers[key]
		}
	}
	a.state = StateEditHeaderValue
	a.overlayTitle = fmt.Sprintf("Edit '%s'", key)
	a.overlayInput.Placeholder = "value"
	a.overlayInput.SetValue(currentVal)
	a.overlayInput.Focus()
	return a, nil
}

func (a App) cycleEnv() (tea.Model, tea.Cmd) {
	if len(a.envFile.Envs) == 0 {
		return a, nil
	}
	for i, e := range a.envFile.Envs {
		if e.Name == a.envFile.Active {
			next := a.envFile.Envs[(i+1)%len(a.envFile.Envs)]
			a.envFile.Active = next.Name
			a.syncActiveEnv()
			a.statusMsg = "Environment: " + next.Name
			return a, nil
		}
	}
	a.envFile.Active = a.envFile.Envs[0].Name
	a.syncActiveEnv()
	return a, nil
}

func (a App) executeRequest() (tea.Model, tea.Cmd) {
	if len(a.collections) == 0 {
		a.statusMsg = "No collections"
		return a, nil
	}
	if a.colIdx >= len(a.collections) {
		return a, nil
	}
	reqs := a.collections[a.colIdx].Requests
	if len(reqs) == 0 || a.reqIdx >= len(reqs) {
		a.statusMsg = "No request selected"
		return a, nil
	}

	req := reqs[a.reqIdx]

	// Resolve environment variables
	req.URL = storage.ResolveEnv(req.URL, a.activeEnv)
	resolved := make(map[string]string)
	for k, v := range req.Headers {
		resolved[k] = storage.ResolveEnv(v, a.activeEnv)
	}
	req.Headers = resolved
	req.Body = storage.ResolveEnv(req.Body, a.activeEnv)

	a.response.SetLoading()
	a.statusMsg = fmt.Sprintf("Sending %s %s…", req.Method, req.URL)

	return a, func() tea.Msg {
		return responseMsg{resp: httpclient.Execute(req)}
	}
}

func (a App) openExternalEditor() (tea.Model, tea.Cmd) {
	body := ""
	if a.colIdx < len(a.collections) {
		reqs := a.collections[a.colIdx].Requests
		if a.reqIdx < len(reqs) {
			body = reqs[a.reqIdx].Body
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "lazyapi-*.json")
	if err != nil {
		a.statusMsg = "Cannot open editor: " + err.Error()
		return a, nil
	}
	f.WriteString(body)
	f.Close()
	fname := f.Name()

	return a, tea.ExecProcess(exec.Command(editor, fname), func(err error) tea.Msg {
		defer os.Remove(fname)
		if err != nil {
			return editorDoneMsg{err: err}
		}
		data, err := os.ReadFile(fname)
		return editorDoneMsg{content: strings.TrimRight(string(data), "\n"), err: err}
	})
}

// ── Sync helpers ──────────────────────────────────────────────────────────────

func (a *App) syncActiveEnv() {
	a.activeEnv = nil
	for i := range a.envFile.Envs {
		if a.envFile.Envs[i].Name == a.envFile.Active {
			a.activeEnv = &a.envFile.Envs[i]
			return
		}
	}
}

func (a *App) syncPanels() {
	// Clamp indices
	if a.colIdx >= len(a.collections) {
		a.colIdx = max(0, len(a.collections)-1)
	}
	if a.colIdx < len(a.collections) {
		reqs := a.collections[a.colIdx].Requests
		if a.reqIdx >= len(reqs) {
			a.reqIdx = max(0, len(reqs)-1)
		}
	}
	a.syncEditor()
}

func (a *App) syncEditor() {
	if a.colIdx < len(a.collections) {
		reqs := a.collections[a.colIdx].Requests
		if a.reqIdx < len(reqs) {
			a.editor.SetRequest(&a.collections[a.colIdx].Requests[a.reqIdx])
			return
		}
	}
	a.editor.SetRequest(nil)
}

func (a *App) syncFocus() {
	a.colPanel.Focused = a.focused == PanelCollections
	a.reqPanel.Focused = a.focused == PanelRequests
	a.editor.Focused = a.focused == PanelEditor
	a.response.Focused = a.focused == PanelResponse
}

func (a App) save() tea.Cmd {
	if a.store == nil {
		return nil
	}
	cols := make([]storage.Collection, len(a.collections))
	copy(cols, a.collections)
	return func() tea.Msg {
		return savedMsg{err: a.store.SaveCollections(cols)}
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	if a.state == StateHelp {
		return ui.RenderHelp(a.width, a.height)
	}

	// Dimensions
	statusH := 1
	topH := (a.height - statusH) * 57 / 100
	if topH < 10 {
		topH = 10
	}
	botH := a.height - statusH - topH
	if botH < 6 {
		botH = 6
		topH = a.height - statusH - botH
	}

	colW := a.width * 18 / 100
	reqW := a.width * 22 / 100
	editorW := a.width - colW - reqW

	// Set panel sizes
	a.colPanel.Width = colW
	a.colPanel.Height = topH
	a.reqPanel.Width = reqW
	a.reqPanel.Height = topH
	a.editor.Width = editorW
	a.editor.Height = topH
	a.response.Width = a.width
	a.response.Height = botH

	// Render panels
	var reqs []storage.Request
	if a.colIdx < len(a.collections) {
		reqs = a.collections[a.colIdx].Requests
	}

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		a.colPanel.View(a.collections),
		a.reqPanel.View(reqs),
		a.editor.View(),
	)
	botRow := a.response.View()

	statusBar := a.renderStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, topRow, botRow, statusBar)
}

func (a App) renderStatusBar() string {
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
		Render(a.mode.String())

	panel := ui.MutedStyle.Render("  " + a.focused.String() + "  ")

	// Environment indicator
	envLabel := ""
	if a.envFile.Active != "" {
		envLabel = lipgloss.NewStyle().
			Foreground(ui.ColorMethodPATCH).
			Render("  [" + a.envFile.Active + "]")
	}

	left := modeLabel + panel + envLabel

	// Center: status message or state hint
	center := ""
	switch a.state {
	case StateNewCollection, StateNewRequest, StateRename:
		center = lipgloss.NewStyle().Foreground(ui.ColorBorderFocused).Render("  "+a.overlayTitle+": ") +
			a.overlayInput.View() +
			ui.MutedStyle.Render("  <Enter> confirm  <Esc> cancel")
	case StateDeleteConfirm:
		center = lipgloss.NewStyle().Foreground(ui.ColorError).Bold(true).Render(
			fmt.Sprintf("  Delete '%s'? [y/n]", a.deleteTarget))
	case StateSearching:
		center = lipgloss.NewStyle().Foreground(ui.ColorBorderFocused).Render("  /") +
			a.searchInput.View() +
			ui.MutedStyle.Render("  <Enter> confirm  <Esc> clear")
	default:
		if a.statusMsg != "" {
			center = lipgloss.NewStyle().Foreground(ui.ColorWarning).Render("  " + a.statusMsg)
		}
	}

	right := ui.MutedStyle.Render(a.hintsFor())

	used := lipgloss.Width(left) + lipgloss.Width(center) + lipgloss.Width(right)
	gap := a.width - used
	if gap < 0 {
		gap = 0
	}
	half := gap / 2

	return ui.StatusBarStyle.Width(a.width).Render(
		left + strings.Repeat(" ", half) + center + strings.Repeat(" ", gap-half) + right,
	)
}

func (a App) hintsFor() string {
	if a.mode == ModeInsert {
		return "Esc·normal "
	}
	switch a.focused {
	case PanelCollections, PanelRequests:
		return "n·new  d·del  e·rename  /·search  r·run  ?·help  q·quit "
	case PanelEditor:
		return "i·url  e·body  m·method  t·tab  r·run  ?·help "
	case PanelResponse:
		return "t·tab  jk·scroll  r·run  ?·help "
	}
	return "?·help  q·quit "
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatSize(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
}
