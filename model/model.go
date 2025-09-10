package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	markdown "github.com/MichaelMure/go-term-markdown"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/electr1fy0/blue/server"
	"github.com/electr1fy0/blue/storage"
	"github.com/electr1fy0/blue/utils"
	"github.com/gorilla/websocket"
)

// Model states
type state int

type WSMessage struct {
	Type     string        `json:"type"`
	Note     *storage.Note `json:"note,omitempty"`
	OldTitle string        `json:"old_title,omitempty"`
}

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

func (i listItem) FilterValue() string { return i.title }

func (i listItem) Title() string {
	prefixParts := []string{}
	if i.pinned {
		prefixParts = append(prefixParts, "(PIN)")
	}
	if i.favorited {
		prefixParts = append(prefixParts, "(FAV)")
	}
	if i.archived {
		prefixParts = append(prefixParts, "(ARC)")
	}
	if len(prefixParts) > 0 {
		return strings.Join(prefixParts, " ") + " " + i.title
	}
	return i.title
}

func (i listItem) Description() string {
	var description string = fmt.Sprintf("Updated: %s", i.updatedAt.Format("2006-01-02 15:04"))
	if len(i.tags) > 0 {
		description += " • tags: " + strings.Join(i.tags, ",")
	}
	return description
}

// helper: render markdown using glamour (glow's rendering engine)
func renderMarkdown(md string, width int) (string, error) {
	if width < 40 {
		width = 40
	}

	// Create glamour renderer with dark theme
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return "", err
	}

	out, err := r.Render(md)
	if err != nil {
		return "", err
	}

	return out, nil
}

// helper: render markdown to ANSI using go-term-markdown (fallback)
func renderMarkdownToANSI(md string, width int) string {
	if width < 40 {
		width = 40
	}
	out := markdown.Render(md, width-4, 4)
	return string(out)
}

// helper: primary renderer using glow (fallback if installed)
func renderWithGlow(md string) (string, error) {
	glowPath, err := exec.LookPath("glow")
	if err != nil {
		return "", fmt.Errorf("glow not found")
	}

	tmp, err := os.CreateTemp("", "note-*.md")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.WriteString(md); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	cmd := exec.Command(glowPath, "-s", "dark", tmpName)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Env = append(os.Environ(), "GLOW_STYLE=dark")

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type Model struct {
	state state

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

func InitialModel() Model {
	// password textinput
	ti := textinput.New()
	ti.Placeholder = "enter password"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	// search input
	si := textinput.New()
	si.Placeholder = "search notes..."
	si.CharLimit = 50
	si.Width = 40

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Notes"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	return Model{
		state:       statePass,
		pwInput:     ti,
		searchInput: si,
		list:        l,
		sortBy:      sortByDate,
		wsStatus:    "disconnected",
	}
}

// ----------------- Bubble Tea implementation -----------------
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// window resize handling
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width - 4)
		m.list.SetHeight(msg.Height - 8)

		// Re-render current note if viewing
		if m.state == stateView && m.current != "" && m.nb != nil {
			if note, exists := m.nb.GetNote(m.current); exists {
				if out, err := renderWithGlow(note.Content); err == nil {
					m.viewContent = out
				} else if out, err := renderMarkdown(note.Content, m.width); err == nil {
					m.viewContent = out
				} else {
					m.viewContent = renderMarkdownToANSI(note.Content, m.width)
				}
			}
		}
	}

	// Handle websocket internal messages first
	switch msg := msg.(type) {
	case server.WsConnected:
		m.ws = msg.Conn
		m.wsStatus = "connected"
		m.status = "Connected to sync server"
	case server.WsError:
		m.wsStatus = "disconnected"
		m.lastError = msg.Err.Error()
		m.status = "WebSocket error: " + msg.Err.Error()
		// If connection failed, nil out the ws so writes don't attempt.
		m.ws = nil
	case server.WsMessage:
		// Only process websocket messages if we have a notebook
		if m.nb == nil {
			return m, nil
		}

		// Attempt to parse a full-sync (array of notes) first
		var notes []storage.Note
		if err := json.Unmarshal(msg.Data, &notes); err == nil && len(notes) > 0 {
			// Replace local notebook notes with server-provided notes
			m.nb.Notes = make(map[string]*storage.Note, len(notes))
			for _, n := range notes {
				nn := n // copy
				m.nb.Notes[nn.Title] = &nn
			}
			m.persist()
			m.refreshList()
			m.status = fmt.Sprintf("Synced %d notes from server", len(notes))
			return m, nil
		}

		// Otherwise parse single WSMessage
		var wmsg WSMessage
		if err := json.Unmarshal(msg.Data, &wmsg); err != nil {
			return m, nil
		}

		switch wmsg.Type {
		case "add":
			n := wmsg.Note
			nn := n
			// if exists, skip; else add
			if _, ok := m.nb.Notes[nn.Title]; !ok {
				m.nb.AddNote(nn)
				m.persist()
				m.refreshList()
				m.status = "Remote add: " + nn.Title
			}
		case "edit":
			n := wmsg.Note
			if wmsg.OldTitle != "" && wmsg.OldTitle != n.Title {
				// remove old key if exists
				delete(m.nb.Notes, wmsg.OldTitle)
			}
			nn := n
			m.nb.Notes[nn.Title] = nn
			m.persist()
			m.refreshList()
			m.status = "Remote edit: " + nn.Title
		case "delete":
			t := wmsg.Note.Title
			if t != "" {
				delete(m.nb.Notes, t)
				m.persist()
				m.refreshList()
				m.status = "Remote delete: " + t
			}
		default:
			// unknown type: ignore
		}
	}

	// main state logic
	switch m.state {
	case statePass:
		var cmd tea.Cmd
		m.pwInput, cmd = m.pwInput.Update(msg)
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.password = m.pwInput.Value()
				exists, err := storage.NotebookExists()
				if err != nil {
					m.status = "Error checking notebook: " + err.Error()
					m.lastError = err.Error()
					return m, nil
				}
				if exists {
					nb, err := storage.LoadNotebook(m.password)
					if err != nil {
						m.status = "Failed to decrypt: " + err.Error()
						m.lastError = err.Error()
						m.pwInput.SetValue("")
						return m, nil
					}
					m.nb = nb
				} else {
					m.nb = storage.NewNotebook()
					if err := storage.SaveNotebook(m.nb, m.password); err != nil {
						m.status = "Failed to create notebook: " + err.Error()
						m.lastError = err.Error()
						return m, nil
					}
				}
				m.refreshList()
				m.state = stateList
				m.status = fmt.Sprintf("Loaded notebook (%d notes)", len(m.nb.Notes))
			}
		}
		return m, cmd

	case stateSearch:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				m.searchTerm = m.searchInput.Value()
				m.refreshList()
				m.state = stateList
				m.status = fmt.Sprintf("Search: '%s' (%d results)", m.searchTerm, len(m.allItems))
			case "esc":
				m.searchTerm = ""
				m.searchInput.SetValue("")
				m.refreshList()
				m.state = stateList
			}
		}
		return m, cmd

	case stateConfirm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				if m.confirmAction != nil {
					m.confirmAction()
				}
				m.state = stateList
			case "n", "N", "esc":
				m.state = stateList
			}
		}
		return m, nil

	case stateList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c", "q":
				// close ws if open
				if m.ws != nil {
					_ = m.ws.Close()
					m.ws = nil
				}
				return m, tea.Quit
			case "/":
				m.searchInput.Focus()
				m.state = stateSearch
			case "c":
				if m.searchTerm != "" {
					m.searchTerm = ""
					m.refreshList()
					m.status = "Cleared search"
				}
			case "s":
				if m.sortBy == sortByTitle {
					m.sortBy = sortByDate
					m.status = "Sorted by date"
				} else {
					m.sortBy = sortByTitle
					m.status = "Sorted by title"
				}
				m.refreshList()
			case "e":
				if err := m.exportNotes(); err != nil {
					m.status = "Export failed: " + err.Error()
					m.lastError = err.Error()
				}
			case "a":
				content, err := utils.OpenEditorWithContent(buildContentWithMeta(noteMeta{}, "# New note\n\nStart writing here...\n"))
				if err != nil {
					m.status = "Editor failed: " + err.Error()
					m.lastError = err.Error()
					break
				}
				title := extractTitle(content)

				// Check for duplicate titles
				originalTitle := title
				counter := 1
				for {
					if _, exists := m.nb.Notes[title]; !exists {
						break
					}
					title = fmt.Sprintf("%s_%d", originalTitle, counter)
					counter++
				}

				n := &storage.Note{
					Title:     title,
					Content:   content,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				m.nb.AddNote(n)
				m.persist()
				m.refreshList()
				m.status = "Added note: " + title

				// send to server if connected
				if m.ws != nil {
					wm := WSMessage{Type: "add", Note: n}
					if err := m.ws.WriteJSON(wm); err != nil {
						m.lastError = err.Error()
						m.status = "WS write failed: " + err.Error()
						_ = m.ws.Close()
						m.ws = nil
						m.wsStatus = "disconnected"
					}
				}
			case "d":
				// Only delete notes if really in list state
				if it := m.list.SelectedItem(); it != nil {
					name := it.(listItem).title
					m.confirmMsg = fmt.Sprintf("Delete note '%s'? (y/N)", name)
					nm := name
					m.confirmAction = func() {
						if m.nb.DeleteNote(nm) {
							m.persist()
							m.refreshList()
							m.status = "Deleted: " + nm
							// notify server
							if m.ws != nil {
								wm := WSMessage{Type: "delete", Note: &storage.Note{Title: nm}}
								if err := m.ws.WriteJSON(wm); err != nil {
									m.lastError = err.Error()
									m.status = "WS write failed: " + err.Error()
									_ = m.ws.Close()
									m.ws = nil
									m.wsStatus = "disconnected"
								}
							}
						}
					}
					m.state = stateConfirm
				}
			case "enter":
				if it := m.list.SelectedItem(); it != nil {
					name := it.(listItem).title
					m.current = name
					note, exists := m.nb.GetNote(name)
					if !exists {
						m.status = "Note not found: " + name
						break
					}
					if out, err := renderWithGlow(note.Content); err == nil {
						m.viewContent = out
					} else if out, err := renderMarkdown(note.Content, m.width); err == nil {
						m.viewContent = out
					} else {
						m.viewContent = renderMarkdownToANSI(note.Content, m.width)
					}
					m.state = stateView
				}
			case "g":
				m.showArchived = !m.showArchived
				if m.showArchived {
					m.status = "Showing archived notes"
				} else {
					m.status = "Showing active notes"
				}
				m.refreshList()
			case "P":
				pi := textinput.New()
				pi.Placeholder = "enter new password"
				pi.Focus()
				pi.CharLimit = 64
				pi.Width = 30
				pi.EchoMode = textinput.EchoPassword
				pi.EchoCharacter = '•'
				m.pwInput = pi
				m.state = stateChangePass
			case "p":
				if it := m.list.SelectedItem(); it != nil {
					name := it.(listItem).title
					m.updateNoteMeta(name, func(meta *noteMeta) {
						meta.Pinned = !meta.Pinned
					})
					m.status = "Toggled pin: " + name
					if m.ws != nil {
						if note, ok := m.nb.GetNote(name); ok {
							wm := WSMessage{Type: "edit", Note: note, OldTitle: note.Title}
							if err := m.ws.WriteJSON(wm); err != nil {
								m.lastError = err.Error()
								m.status = "WS write failed: " + err.Error()
								_ = m.ws.Close()
								m.ws = nil
								m.wsStatus = "disconnected"
							}
						}
					}
				}
			case "f":
				if it := m.list.SelectedItem(); it != nil {
					name := it.(listItem).title
					m.updateNoteMeta(name, func(meta *noteMeta) {
						meta.Favorite = !meta.Favorite
					})
					m.status = "Toggled favorite: " + name
					if m.ws != nil {
						if note, ok := m.nb.GetNote(name); ok {
							wm := WSMessage{Type: "edit", Note: note, OldTitle: note.Title}
							if err := m.ws.WriteJSON(wm); err != nil {
								m.lastError = err.Error()
								m.status = "WS write failed: " + err.Error()
								_ = m.ws.Close()
								m.ws = nil
								m.wsStatus = "disconnected"
							}
						}
					}
				}
			case "t":
				if it := m.list.SelectedItem(); it != nil {
					name := it.(listItem).title
					if note, ok := m.nb.GetNote(name); ok {
						meta, body := parseFrontMatter(note.Content)
						initial := "tags: " + strings.Join(meta.Tags, ",") + "\n\n# edit tags as comma-separated values above\n"
						out, err := utils.OpenEditorWithContent(initial)
						if err != nil {
							m.status = "Editor failed: " + err.Error()
							m.lastError = err.Error()
							break
						}
						lines := strings.Split(out, "\n")
						if len(lines) > 0 {
							first := strings.TrimSpace(lines[0])
							if strings.HasPrefix(strings.ToLower(first), "tags:") {
								val := strings.TrimSpace(first[len("tags:"):])
								val = strings.Trim(val, "[] ")
								if val == "" {
									meta.Tags = []string{}
								} else {
									parts := strings.Split(val, ",")
									for i := range parts {
										parts[i] = strings.TrimSpace(parts[i])
									}
									meta.Tags = parts
								}
								// rebuild content with the same body
								note.Content = buildContentWithMeta(meta, body)
								note.UpdatedAt = time.Now()
								m.persist()
								m.refreshList()
								m.status = "Updated tags for " + name
							}
						}
					}
				}
			}
		}
		return m, cmd

	case stateView:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c", "q":
				// close ws if open
				if m.ws != nil {
					_ = m.ws.Close()
					m.ws = nil
				}
				return m, tea.Quit
			case "b", "esc":
				m.state = stateList
			case "e":
				note, exists := m.nb.GetNote(m.current)
				if !exists {
					m.status = "Note not found: " + m.current
					m.state = stateList
					break
				}
				content, err := utils.OpenEditorWithContent(note.Content)
				if err != nil {
					m.status = "Editor failed: " + err.Error()
					m.lastError = err.Error()
					break
				}

				// Update note
				oldTitle := note.Title
				note.Content = content
				note.UpdatedAt = time.Now()

				newTitle := extractTitle(content)
				if newTitle != oldTitle {
					delete(m.nb.Notes, oldTitle)
					note.Title = newTitle
					m.nb.Notes[newTitle] = note
					m.current = newTitle
				} else {
					m.nb.Notes[m.current] = note
				}

				m.persist()
				m.refreshList()

				if rendered, err := renderMarkdown(note.Content, m.width); err == nil {
					m.viewContent = rendered
				} else {
					m.viewContent = "Error rendering markdown: " + err.Error()
				}
				m.status = "Edited " + m.current

				// send edit to server
				if m.ws != nil {
					wm := WSMessage{
						Type:     "edit",
						Note:     note,
						OldTitle: oldTitle,
					}
					if err := m.ws.WriteJSON(wm); err != nil {
						m.lastError = err.Error()
						m.status = "WS write failed: " + err.Error()
						_ = m.ws.Close()
						m.ws = nil
						m.wsStatus = "disconnected"
					}
				}
			case "d":
				// Only allow delete when in view mode
				m.confirmMsg = fmt.Sprintf("Delete note '%s'? (y/N)", m.current)
				cur := m.current
				m.confirmAction = func() {
					if m.nb.DeleteNote(cur) {
						m.persist()
						m.refreshList()
						m.status = "Deleted: " + cur
						m.state = stateList
						// notify server
						if m.ws != nil {
							wm := WSMessage{Type: "delete", Note: &storage.Note{Title: cur}}
							if err := m.ws.WriteJSON(wm); err != nil {
								m.lastError = err.Error()
								m.status = "WS write failed: " + err.Error()
								_ = m.ws.Close()
								m.ws = nil
								m.wsStatus = "disconnected"
							}
						}
					}
				}
				m.state = stateConfirm
			case "p":
				// toggle pin for current
				name := m.current
				m.updateNoteMeta(name, func(meta *noteMeta) {
					meta.Pinned = !meta.Pinned
				})
				m.status = "Toggled pin: " + name
			case "f":
				name := m.current
				m.updateNoteMeta(name, func(meta *noteMeta) {
					meta.Favorite = !meta.Favorite
				})
				m.status = "Toggled favorite: " + name
			case "t":
				// edit tags
				name := m.current
				if note, ok := m.nb.GetNote(name); ok {
					meta, _ := parseFrontMatter(note.Content)
					initial := "tags: " + strings.Join(meta.Tags, ",") + "\n\n# edit tags as comma-separated values above\n"
					out, err := utils.OpenEditorWithContent(initial)
					if err != nil {
						m.status = "Editor failed: " + err.Error()
						m.lastError = err.Error()
						break
					}
					lines := strings.Split(out, "\n")
					if len(lines) > 0 {
						first := strings.TrimSpace(lines[0])
						if strings.HasPrefix(strings.ToLower(first), "tags:") {
							val := strings.TrimSpace(first[len("tags:"):])
							val = strings.Trim(val, "[] ")
							if val == "" {
								meta.Tags = []string{}
							} else {
								parts := strings.Split(val, ",")
								for i := range parts {
									parts[i] = strings.TrimSpace(parts[i])
								}
								meta.Tags = parts
							}
							_, body2 := parseFrontMatter(note.Content)
							note.Content = buildContentWithMeta(meta, body2)
							note.UpdatedAt = time.Now()
							m.persist()
							m.refreshList()
							m.status = "Updated tags for " + name
						}
					}
				}
			case "r":
				// archive/unarchive
				name := m.current
				m.updateNoteMeta(name, func(meta *noteMeta) {
					meta.Archived = !meta.Archived
				})
				m.status = "Toggled archive: " + name
			}
		}
		return m, nil
	case stateChangePass:
		var cmd tea.Cmd
		m.pwInput, cmd = m.pwInput.Update(msg)
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				newpw := m.pwInput.Value()
				if newpw == "" {
					m.status = "Password not changed (empty)"
					m.state = stateList
					break
				}
				if err := m.changePassword(newpw); err != nil {
					m.status = "Password change failed: " + err.Error()
					m.lastError = err.Error()
				} else {
					m.status = "Password changed."
				}
				// reset pw input to masked blank for future use
				pi := textinput.New()
				pi.Placeholder = "enter password"
				pi.Focus()
				pi.CharLimit = 64
				pi.Width = 30
				pi.EchoMode = textinput.EchoPassword
				pi.EchoCharacter = '•'
				m.pwInput = pi
				m.state = stateList
			case "esc":
				m.state = stateList
			}
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var s strings.Builder
	s.WriteString(titleStyle.Render("blue — secure notes"))
	s.WriteString("\n\n")

	switch m.state {
	case statePass:
		s.WriteString("Enter password to unlock/create notebook:\n\n")
		s.WriteString(m.pwInput.View())
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("ctrl+c: quit"))
		if m.status != "" {
			s.WriteString("\n")
			if m.lastError != "" {
				s.WriteString(errorStyle.Render(m.status))
			} else {
				s.WriteString(helpStyle.Render(m.status))
			}
		}

	case stateSearch:
		s.WriteString("Search notes:\n\n")
		s.WriteString(m.searchInput.View())
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("enter: search  esc: cancel"))

	case stateConfirm:
		s.WriteString(warningStyle.Render(m.confirmMsg))
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("y: confirm  n/esc: cancel"))

	case stateList:
		s.WriteString(m.list.View())
		s.WriteString("\n")

		// Build help text
		var helpParts []string
		helpParts = append(helpParts, "a:add", "d:delete", "enter:view")
		helpParts = append(helpParts, "/:search")
		if m.searchTerm != "" {
			helpParts = append(helpParts, "c:clear search")
		}
		helpParts = append(helpParts, "s:sort", "e:export", "g:toggle archived", "P:change password", "q:quit")
		// quick metadata actions
		helpParts = append(helpParts, "p:pin", "f:favorite", "t:tags")
		s.WriteString(helpStyle.Render(strings.Join(helpParts, "  ")))

		// Show current sort and search status
		var statusParts []string
		if m.sortBy == sortByTitle {
			statusParts = append(statusParts, "sorted by title")
		} else {
			statusParts = append(statusParts, "sorted by date")
		}
		if m.searchTerm != "" {
			statusParts = append(statusParts, fmt.Sprintf("search: '%s'", m.searchTerm))
		}
		if m.showArchived {
			statusParts = append(statusParts, "viewing archived")
		}
		if len(statusParts) > 0 {
			s.WriteString("\n")
			s.WriteString(helpStyle.Render(strings.Join(statusParts, " • ")))
		}

		// WebSocket status
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("WS: " + m.wsStatus))

		if m.status != "" {
			s.WriteString("\n")
			if m.lastError != "" {
				s.WriteString(errorStyle.Render(m.status))
			} else {
				s.WriteString(successStyle.Render(m.status))
			}
		}

	case stateView:
		s.WriteString(titleStyle.Render(m.current))
		s.WriteString("\n\n")
		s.WriteString(m.viewContent)
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("e:edit  d:delete  b:back  q:quit"))
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("p:pin  f:favorite  t:tags  r:archive"))
		if m.status != "" {
			s.WriteString("\n")
			if m.lastError != "" {
				s.WriteString(errorStyle.Render(m.status))
			} else {
				s.WriteString(successStyle.Render(m.status))
			}
		}
	}

	return s.String()
}

// ------------------ front matter helpers ------------------
// parseFrontMatter extracts meta and the body (without front matter).
// If no front matter present, returns default meta and whole body.
func parseFrontMatter(content string) (noteMeta, string) {
	meta := noteMeta{}
	trim := strings.TrimLeft(content, "\n\r\t ")
	if !strings.HasPrefix(trim, "---") {
		// no front matter
		return meta, content
	}
	// find the closing '---' on its own line
	// search for "\n---" after the first line
	// handle case like "---\nkey: val\n---\n"
	rest := trim[3:]
	idx := strings.Index(rest, "---")
	if idx == -1 {
		// malformed - treat as no front matter
		return meta, content
	}

	metaBlock := rest[:idx]
	// body starts after the closing '---'
	body := strings.TrimLeft(rest[idx+3:], "\n\r")

	// parse lines like "tags: a,b" or "pinned: true"
	lines := strings.SplitSeq(metaBlock, "\n")
	for ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		parts := strings.SplitN(ln, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(strings.ToLower(parts[0]))
		v := strings.TrimSpace(parts[1])
		switch k {
		case "tags":
			v = strings.Trim(v, "[] ")
			if v == "" {
				meta.Tags = []string{}
			} else if strings.Contains(v, ",") {
				ps := strings.Split(v, ",")
				for i := range ps {
					ps[i] = strings.TrimSpace(ps[i])
				}
				meta.Tags = ps
			} else {
				// split by spaces
				ps := strings.Fields(v)
				meta.Tags = ps
			}
		case "pinned":
			b, _ := strconv.ParseBool(v)
			meta.Pinned = b
		case "favorite", "favorited":
			b, _ := strconv.ParseBool(v)
			meta.Favorite = b
		case "archived":
			b, _ := strconv.ParseBool(v)
			meta.Archived = b
		default:
			// ignore unknown keys
		}
	}
	return meta, body
}

func buildContentWithMeta(meta noteMeta, body string) string {
	lines := []string{"---"}
	if len(meta.Tags) > 0 {
		lines = append(lines, "tags: "+strings.Join(meta.Tags, ","))
	} else {
		lines = append(lines, "tags: ")
	}
	lines = append(lines, fmt.Sprintf("pinned: %t", meta.Pinned))
	lines = append(lines, fmt.Sprintf("favorite: %t", meta.Favorite))
	lines = append(lines, fmt.Sprintf("archived: %t", meta.Archived))
	lines = append(lines, "---", "", body)
	return strings.Join(lines, "\n")
}

// helper: quick update meta for a named note, save and refresh UI
func (m *Model) updateNoteMeta(title string, updater func(*noteMeta)) {
	if m.nb == nil {
		return
	}
	n, ok := m.nb.GetNote(title)
	if !ok {
		return
	}
	meta, body := parseFrontMatter(n.Content)
	updater(&meta)
	n.Content = buildContentWithMeta(meta, body)
	n.UpdatedAt = time.Now()

	// ensure key in map is current title
	m.nb.Notes[n.Title] = n
	m.persist()
	m.refreshList()
}

// ----------------- end front matter helpers -----------------

// helper: populate list from notebook with optional search filter
func (m *Model) refreshList() {
	if m.nb == nil {
		return
	}
	items := make([]list.Item, 0, len(m.nb.Notes))

	for title, note := range m.nb.Notes {
		// parse meta
		meta, _ := parseFrontMatter(note.Content)

		// archived filtering
		if meta.Archived && !m.showArchived {
			continue
		}
		if !meta.Archived && m.showArchived {
			// when viewing archives only, skip non-archived
			continue
		}

		// apply search filter if active
		if m.searchTerm != "" {
			if !strings.Contains(strings.ToLower(title), strings.ToLower(m.searchTerm)) {
				_, body := parseFrontMatter(note.Content)
				if !strings.Contains(strings.ToLower(body), strings.ToLower(m.searchTerm)) {
					continue
				}
			}
		}
		items = append(items, listItem{
			title:     title,
			updatedAt: note.UpdatedAt,
			tags:      meta.Tags,
			pinned:    meta.Pinned,
			favorited: meta.Favorite,
			archived:  meta.Archived,
		})
	}

	// sort items: pinned first, then favorited, then by chosen sort
	sort.Slice(items, func(i, j int) bool {
		li, ok1 := items[i].(listItem)
		lj, ok2 := items[j].(listItem)
		if !ok1 || !ok2 {
			return false
		}
		// pinned first
		if li.pinned != lj.pinned {
			return li.pinned
		}
		// favorited next
		if li.favorited != lj.favorited {
			return li.favorited
		}
		switch m.sortBy {
		case sortByTitle:
			return li.title < lj.title
		case sortByDate:
			return li.updatedAt.After(lj.updatedAt)
		}
		return false
	})

	m.allItems = items
	m.list.SetItems(items)
}

func extractTitle(content string) string {
	// strip front matter
	_, body := parseFrontMatter(content)
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		// Remove markdown heading prefix
		if strings.HasPrefix(trim, "# ") {
			return strings.TrimSpace(trim[2:])
		}
		// Use first non-empty line as title, truncated if too long
		if len(trim) > 50 {
			return trim[:47] + "..."
		}
		return trim
	}
	return fmt.Sprintf("Note_%d", time.Now().Unix())
}

func (m *Model) persist() {
	if err := storage.SaveNotebook(m.nb, m.password); err != nil {
		m.status = "Failed to save: " + err.Error()
		m.lastError = err.Error()
		return
	}
	m.status = "Saved."
	m.lastError = ""
}
