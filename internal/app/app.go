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

// ── messages ─────────────────────────────────────────────────────────────────

type responseMsg struct{ resp storage.Response }
type savedMsg struct{ err error }
type editorDoneMsg struct {
	content string
	err     error
}

// ── App ───────────────────────────────────────────────────────────────────────

type App struct {
	mode    Mode
	state   AppState
	focused Panel

	ggPressed bool

	width, height int

	// Data
	collections []storage.Collection
	reqIdx      int
	envFile     storage.EnvFile
	activeEnv   *storage.Environment

	// Panels
	colPanel ui.CollectionsPanel
	reqPanel ui.RequestsPanel
	editor   ui.EditorPanel
	response ui.ResponsePanel

	// Overlay
	overlayInput     textinput.Model
	overlayTitle     string
	deleteTarget     string
	pendingHeaderKey string

	// Search
	searchInput textinput.Model

	store     *storage.Store
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
		colPanel:     ui.NewCollectionsPanel(),
		editor:       ui.NewEditorPanel(),
		response:     ui.NewResponsePanel(),
		overlayInput: overlayIn,
		searchInput:  searchIn,
	}

	store, err := storage.NewStore()
	if err == nil {
		a.store = store
		if cols, err := store.LoadCollections(); err == nil {
			a.collections = cols
		}
		if ef, err := store.LoadEnvFile(); err == nil {
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
	a.syncEditor()
	a.syncFocus()
	return a
}

func (a App) Init() tea.Cmd { return nil }

// ── Update ────────────────────────────────────────────────────────────────────

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
			a.statusMsg = fmt.Sprintf("%s  ·  %dms  ·  %s", r.Status, r.DurationMs, formatSize(r.SizeBytes))
		}
		return a, nil

	case editorDoneMsg:
		if msg.err == nil {
			col := a.currentCollection()
			if col != nil && a.reqIdx < len(col.Requests) {
				col.Requests[a.reqIdx].Body = msg.content
				a.editor.SetRequest(&col.Requests[a.reqIdx])
				return a, a.save()
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

// ── Normal mode ───────────────────────────────────────────────────────────────

func (a App) handleNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

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

	// Actions (with panel/tab-specific overrides)
	switch k {
	case "r":
		return a.executeRequest()
	case "enter":
		if a.focused == PanelCollections {
			a.colPanel.Toggle(a.collections)
			a.syncEditor()
			return a, nil
		}
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
			col := a.currentCollection()
			if col != nil && a.reqIdx < len(col.Requests) {
				col.Requests[a.reqIdx].Method = a.editor.Request.Method
				return a, a.save()
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
		if a.focused == PanelCollections {
			if a.colPanel.SelectedHasKids(a.collections) && !a.colPanel.Expanded[a.currentColID()] {
				// expand in place instead of jumping
				a.colPanel.Toggle(a.collections)
			} else {
				a.focused = PanelRequests
				a.syncFocus()
			}
		} else if a.focused == PanelRequests {
			a.focused = PanelEditor
			a.syncFocus()
		} else if a.focused == PanelEditor {
			a.focused = PanelResponse
			a.syncFocus()
		}
	case "h", "left":
		if a.focused == PanelCollections {
			a.colPanel.CollapseToParent(a.collections)
			a.syncEditor()
		} else if a.focused == PanelResponse {
			a.focused = PanelEditor
			a.syncFocus()
		} else if a.focused == PanelEditor {
			a.focused = PanelRequests
			a.syncFocus()
		} else if a.focused == PanelRequests {
			a.focused = PanelCollections
			a.syncFocus()
		}
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
		col := a.currentCollection()
		if col != nil && a.reqIdx < len(col.Requests) {
			col.Requests[a.reqIdx].URL = a.editor.URLInput.Value()
			return a, a.save()
		}
		return a, nil
	}
	var cmd tea.Cmd
	a.editor.URLInput, cmd = a.editor.URLInput.Update(msg)
	return a, cmd
}

// ── Overlay ───────────────────────────────────────────────────────────────────

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

	switch a.state {
	case StateNewCollection:
		val := strings.TrimSpace(a.overlayInput.Value())
		if val != "" {
			newCol := storage.Collection{ID: storage.NewID(), Name: val}
			// Add at same level as current selection
			path := a.colPanel.SelectedPath(a.collections)
			if len(path) <= 1 {
				a.collections = append(a.collections, newCol)
			} else {
				parentPath := path[:len(path)-1]
				parent := getCollByPath(a.collections, parentPath)
				if parent != nil {
					parent.Children = append(parent.Children, newCol)
				}
			}
		}
		a.state = StateDefault
		a.overlayInput.Blur()
		return a, a.save()

	case StateNewRequest:
		val := strings.TrimSpace(a.overlayInput.Value())
		if val != "" {
			col := a.currentCollection()
			if col != nil {
				newReq := storage.Request{ID: storage.NewID(), Name: val, Method: "GET"}
				col.Requests = append(col.Requests, newReq)
				a.reqIdx = len(col.Requests) - 1
				a.syncEditor()
			}
		}
		a.state = StateDefault
		a.overlayInput.Blur()
		return a, a.save()

	case StateRename:
		val := strings.TrimSpace(a.overlayInput.Value())
		if val != "" {
			switch a.focused {
			case PanelCollections:
				col := a.currentCollection()
				if col != nil {
					col.Name = val
				}
			case PanelRequests:
				col := a.currentCollection()
				if col != nil && a.reqIdx < len(col.Requests) {
					col.Requests[a.reqIdx].Name = val
				}
			}
		}
		a.state = StateDefault
		a.overlayInput.Blur()
		return a, a.save()

	case StateNewHeaderKey:
		key := strings.TrimSpace(a.overlayInput.Value())
		if key == "" {
			a.state = StateDefault
			a.overlayInput.Blur()
			return a, nil
		}
		a.pendingHeaderKey = key
		a.state = StateNewHeaderValue
		a.overlayTitle = fmt.Sprintf("Value for '%s'", key)
		a.overlayInput.Placeholder = "header value"
		a.overlayInput.SetValue("")
		return a, nil

	case StateNewHeaderValue:
		col := a.currentCollection()
		if col != nil && a.reqIdx < len(col.Requests) {
			if col.Requests[a.reqIdx].Headers == nil {
				col.Requests[a.reqIdx].Headers = make(map[string]string)
			}
			col.Requests[a.reqIdx].Headers[a.pendingHeaderKey] = a.overlayInput.Value()
			a.editor.SetRequest(&col.Requests[a.reqIdx])
		}
		a.pendingHeaderKey = ""
		a.state = StateDefault
		a.overlayInput.Blur()
		return a, a.save()

	case StateEditHeaderValue:
		hKey := a.editor.SelectedHeaderKey()
		if hKey != "" {
			col := a.currentCollection()
			if col != nil && a.reqIdx < len(col.Requests) {
				col.Requests[a.reqIdx].Headers[hKey] = a.overlayInput.Value()
				a.editor.SetRequest(&col.Requests[a.reqIdx])
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
			path := a.colPanel.SelectedPath(a.collections)
			if len(path) > 0 {
				a.collections = deleteCollAt(a.collections, path)
				// Clamp cursor
				nodes := ui.BuildTree(a.collections, a.colPanel.Expanded, 0, nil)
				if a.colPanel.Cursor >= len(nodes) && len(nodes) > 0 {
					a.colPanel.Cursor = len(nodes) - 1
				}
				a.reqIdx = 0
			}
		case PanelRequests:
			col := a.currentCollection()
			if col != nil && a.reqIdx < len(col.Requests) {
				col.Requests = append(col.Requests[:a.reqIdx], col.Requests[a.reqIdx+1:]...)
				if a.reqIdx > 0 {
					a.reqIdx--
				}
			}
		}
		a.state = StateDefault
		a.syncEditor()
		return a, a.save()
	case "n", "N", "esc":
		a.state = StateDefault
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
		} else {
			a.reqPanel.SetFilter(filter)
			if msg.String() == "enter" {
				col := a.currentCollection()
				if col != nil {
					idx := a.reqPanel.SelectedIdx(col.Requests)
					if idx >= 0 {
						a.reqIdx = idx
					}
				}
			}
		}
		a.syncEditor()
		return a, nil
	}
	var cmd tea.Cmd
	a.searchInput, cmd = a.searchInput.Update(msg)
	return a, cmd
}

// ── Navigation ────────────────────────────────────────────────────────────────

func (a *App) moveDown() {
	switch a.focused {
	case PanelCollections:
		a.colPanel.MoveDown(a.collections)
		a.reqIdx = 0
		a.reqPanel.Cursor = 0
		a.syncEditor()
	case PanelRequests:
		col := a.currentCollection()
		if col != nil {
			a.reqPanel.MoveDown(col.Requests)
			idx := a.reqPanel.SelectedIdx(col.Requests)
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
		a.reqIdx = 0
		a.reqPanel.Cursor = 0
		a.syncEditor()
	case PanelRequests:
		a.reqPanel.MoveUp()
		col := a.currentCollection()
		if col != nil {
			idx := a.reqPanel.SelectedIdx(col.Requests)
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
		a.reqIdx = 0
		a.reqPanel.Cursor = 0
		a.syncEditor()
	case PanelRequests:
		col := a.currentCollection()
		if col != nil {
			a.reqPanel.GoBottom(col.Requests)
			idx := a.reqPanel.SelectedIdx(col.Requests)
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
		if a.currentCollection() == nil {
			a.statusMsg = "Select a collection first"
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
		col := a.currentCollection()
		if col != nil {
			a.deleteTarget = col.Name
			a.state = StateDeleteConfirm
		}
	case PanelRequests:
		col := a.currentCollection()
		if col != nil && a.reqIdx < len(col.Requests) {
			a.deleteTarget = col.Requests[a.reqIdx].Name
			a.state = StateDeleteConfirm
		}
	}
	return a, nil
}

func (a App) startRename() (tea.Model, tea.Cmd) {
	var current string
	switch a.focused {
	case PanelCollections:
		col := a.currentCollection()
		if col != nil {
			current = col.Name
		}
	case PanelRequests:
		col := a.currentCollection()
		if col != nil && a.reqIdx < len(col.Requests) {
			current = col.Requests[a.reqIdx].Name
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
	col := a.currentCollection()
	if col != nil && a.reqIdx < len(col.Requests) {
		delete(col.Requests[a.reqIdx].Headers, key)
		a.editor.SetRequest(&col.Requests[a.reqIdx])
		if a.editor.HeaderCursor > 0 {
			a.editor.HeaderCursor--
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
	col := a.currentCollection()
	if col != nil && a.reqIdx < len(col.Requests) {
		currentVal = col.Requests[a.reqIdx].Headers[key]
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
	col := a.currentCollection()
	if col == nil || len(col.Requests) == 0 {
		a.statusMsg = "No request selected"
		return a, nil
	}
	if a.reqIdx >= len(col.Requests) {
		a.statusMsg = "No request selected"
		return a, nil
	}

	req := col.Requests[a.reqIdx]
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
	col := a.currentCollection()
	if col != nil && a.reqIdx < len(col.Requests) {
		body = col.Requests[a.reqIdx].Body
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

// ── Tree helpers ──────────────────────────────────────────────────────────────

// getCollByPath walks the path and returns a pointer to the collection.
// Modifications through the pointer propagate to a.collections.
func getCollByPath(cols []storage.Collection, path []int) *storage.Collection {
	if len(path) == 0 || path[0] >= len(cols) {
		return nil
	}
	if len(path) == 1 {
		return &cols[path[0]]
	}
	return getCollByPath(cols[path[0]].Children, path[1:])
}

// deleteCollAt removes the collection at path and returns the updated root slice.
func deleteCollAt(cols []storage.Collection, path []int) []storage.Collection {
	if len(path) == 0 || path[0] >= len(cols) {
		return cols
	}
	if len(path) == 1 {
		return append(cols[:path[0]], cols[path[0]+1:]...)
	}
	cols[path[0]].Children = deleteCollAt(cols[path[0]].Children, path[1:])
	return cols
}

func (a App) currentCollection() *storage.Collection {
	return getCollByPath(a.collections, a.colPanel.SelectedPath(a.collections))
}

func (a App) currentColID() string {
	col := a.currentCollection()
	if col == nil {
		return ""
	}
	return col.ID
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

func (a *App) syncEditor() {
	col := a.currentCollection()
	if col == nil || len(col.Requests) == 0 {
		a.editor.SetRequest(nil)
		return
	}
	if a.reqIdx >= len(col.Requests) {
		a.reqIdx = max(0, len(col.Requests)-1)
	}
	a.editor.SetRequest(&col.Requests[a.reqIdx])
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

	main := a.renderMain()

	switch a.state {
	case StateNewCollection, StateNewRequest, StateRename,
		StateNewHeaderKey, StateNewHeaderValue, StateEditHeaderValue:
		popup := ui.InputPopup(a.overlayTitle, "Enter · confirm     Esc · cancel", a.overlayInput)
		return ui.PlaceOverlay(main, popup, a.width, a.height)
	case StateDeleteConfirm:
		popup := ui.ConfirmPopup(a.deleteTarget)
		return ui.PlaceOverlay(main, popup, a.width, a.height)
	}

	return main
}

func (a App) renderMain() string {
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

	a.colPanel.Width = colW
	a.colPanel.Height = topH
	a.reqPanel.Width = reqW
	a.reqPanel.Height = topH
	a.editor.Width = editorW
	a.editor.Height = topH
	a.response.Width = a.width
	a.response.Height = botH

	var reqs []storage.Request
	if col := a.currentCollection(); col != nil {
		reqs = col.Requests
	}

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		a.colPanel.View(a.collections),
		a.reqPanel.View(reqs),
		a.editor.View(),
	)
	return lipgloss.JoinVertical(lipgloss.Left, topRow, a.response.View(), a.renderStatusBar())
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
		Bold(true).Padding(0, 1).
		Render(a.mode.String())

	panel := ui.MutedStyle.Render("  " + a.focused.String() + "  ")

	envLabel := ""
	if a.envFile.Active != "" {
		envLabel = lipgloss.NewStyle().Foreground(ui.ColorMethodPATCH).
			Render("  [" + a.envFile.Active + "]")
	}
	left := modeLabel + panel + envLabel

	center := ""
	switch a.state {
	case StateSearching:
		center = lipgloss.NewStyle().Foreground(ui.ColorBorderFocused).Render("  /") +
			a.searchInput.View() +
			ui.MutedStyle.Render("  Enter · confirm   Esc · clear")
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
	case PanelCollections:
		return "Enter·expand  l·requests  n·new  d·del  e·rename  /·search  q·quit "
	case PanelRequests:
		return "n·new  d·del  e·rename  /·search  r·run  ?·help "
	case PanelEditor:
		return "i·url  e·body  m·method  t·tab  r·run  ?·help "
	case PanelResponse:
		return "t·tab  jk·scroll  r·run  ?·help "
	}
	return "?·help  q·quit "
}

// ── Utilities ─────────────────────────────────────────────────────────────────

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
