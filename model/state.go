package model

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/electr1fy0/blue/storage"
	"github.com/gorilla/websocket"
)

type noteMeta struct {
	Tags     []string
	Pinned   bool
	Favorite bool
	Archived bool
}

const (
	statePass state = iota
	stateList
	stateView
	stateSearch
	stateConfirm
	stateQuit
	stateChangePass
)

// sort options
type sortMode int

const (
	sortByTitle sortMode = iota
	sortByDate
)

type listItem struct {
	title     string
	updatedAt time.Time
	tags      []string
	pinned    bool
	favorited bool
	archived  bool
}

type state int

type WSMessage struct {
	Type     string        `json:"type"`
	Note     *storage.Note `json:"note,omitempty"`
	OldTitle string        `json:"old_title,omitempty"`
}

type Model struct {
	state       state
	renderCache map[string]string

	// terminal dimensions
	width  int
	height int

	// password prompt
	pwInput  textinput.Model
	password string

	// notebook
	nb *storage.Notebook

	// list UI
	list   list.Model
	sortBy sortMode

	// search
	searchInput textinput.Model
	searchTerm  string
	allItems    []list.Item

	// currently selected note for viewing
	current     string
	viewContent string

	// confirmation dialog
	confirmMsg    string
	confirmAction func()

	// status
	status    string
	lastError string

	// websocket
	ws       *websocket.Conn
	wsStatus string

	// new additions
	showArchived bool
}
