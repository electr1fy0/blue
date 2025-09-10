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

	width  int
	height int

	pwInput  textinput.Model
	password string

	nb *storage.Notebook

	list   list.Model
	sortBy sortMode

	searchInput textinput.Model
	searchTerm  string
	allItems    []list.Item

	current     string
	viewContent string

	confirmMsg    string
	confirmAction func()

	status    string
	lastError string

	ws       *websocket.Conn
	wsStatus string

	showArchived bool
}
