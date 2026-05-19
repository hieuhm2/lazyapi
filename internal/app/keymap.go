package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	// Panel navigation
	NextPanel key.Binding
	PrevPanel key.Binding
	Panel1    key.Binding
	Panel2    key.Binding
	Panel3    key.Binding
	Panel4    key.Binding

	// List navigation (normal mode)
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Top      key.Binding
	Bottom   key.Binding
	HalfUp   key.Binding
	HalfDown key.Binding

	// Actions
	Execute key.Binding
	New     key.Binding
	Delete  key.Binding
	Edit    key.Binding
	Yank    key.Binding

	// Mode switching
	Insert  key.Binding
	Escape  key.Binding
	Command key.Binding

	// App
	Help key.Binding
	Quit key.Binding
}

var DefaultKeyMap = KeyMap{
	NextPanel: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next panel")),
	PrevPanel: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev panel")),
	Panel1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "collections")),
	Panel2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "requests")),
	Panel3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "editor")),
	Panel4:    key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "response")),

	Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	Left:     key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "left")),
	Right:    key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "right")),
	Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "top")),
	Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	HalfUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("C-u", "half up")),
	HalfDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("C-d", "half down")),

	Execute: key.NewBinding(key.WithKeys("enter", "r"), key.WithHelp("enter/r", "execute")),
	New:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Edit:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit name")),
	Yank:    key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy URL")),

	Insert:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "insert mode")),
	Escape:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "normal mode")),
	Command: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),

	Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
