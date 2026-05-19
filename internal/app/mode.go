package app

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
)

func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeCommand:
		return "COMMAND"
	default:
		return "NORMAL"
	}
}

type Panel int

const (
	PanelCollections Panel = iota
	PanelRequests
	PanelEditor
	PanelResponse
	PanelCount
)

func (p Panel) String() string {
	switch p {
	case PanelCollections:
		return "Collections"
	case PanelRequests:
		return "Requests"
	case PanelEditor:
		return "Editor"
	case PanelResponse:
		return "Response"
	default:
		return ""
	}
}

type AppState int

const (
	StateDefault AppState = iota
	StateHelp
	StateNewCollection
	StateNewRequest
	StateRename
	StateDeleteConfirm
	StateSearching
)
