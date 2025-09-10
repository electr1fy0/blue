package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/electr1fy0/blue/server"
	"github.com/electr1fy0/blue/storage"
	"github.com/electr1fy0/blue/utils"
)

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

func renderMarkdown(md string, width int) (string, error) {
	if width < 40 {
		width = 40
	}

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

func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "enter password"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

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

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width - 4)
		m.list.SetHeight(msg.Height - 8)

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

	switch msg := msg.(type) {
	case server.WsConnected:
		m.ws = msg.Conn
		m.wsStatus = "connected"
		m.status = "Connected to sync server"
	case server.WsError:
		m.wsStatus = "disconnected"
		m.lastError = msg.Err.Error()
		m.status = "WebSocket error: " + msg.Err.Error()

		m.ws = nil
	case server.WsMessage:

		if m.nb == nil {
			return m, nil
		}

		var notes []storage.Note
		if err := json.Unmarshal(msg.Data, &notes); err == nil && len(notes) > 0 {

			m.nb.Notes = make(map[string]*storage.Note, len(notes))
			for _, n := range notes {
				nn := n
				m.nb.Notes[nn.Title] = &nn
			}
			m.persist()
			m.refreshList()
			m.status = fmt.Sprintf("Synced %d notes from server", len(notes))
			return m, nil
		}

		var wmsg WSMessage
		if err := json.Unmarshal(msg.Data, &wmsg); err != nil {
			return m, nil
		}

		switch wmsg.Type {
		case "add":
			n := wmsg.Note
			nn := n

			if _, ok := m.nb.Notes[nn.Title]; !ok {
				m.nb.AddNote(nn)
				m.persist()
				m.refreshList()
				m.status = "Remote add: " + nn.Title
			}
		case "edit":
			n := wmsg.Note
			if wmsg.OldTitle != "" && wmsg.OldTitle != n.Title {

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
		}
	}

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
					return m, tea.ClearScreen
				}

				content, err := utils.OpenEditorWithContent(note.Content)
				if err != nil {
					m.status = "Editor failed: " + err.Error()
					m.lastError = err.Error()
					return m, tea.ClearScreen
				}

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

				if m.width > 0 && m.height > 0 {
					m.list.SetWidth(m.width - 4)
					m.list.SetHeight(m.height - 8)
				}

				m.status = "Edited " + m.current

				m.state = stateList
				return m, tea.ClearScreen
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

							if m.width > 0 && m.height > 0 {
								m.list.SetWidth(m.width - 4)
								m.list.SetHeight(m.height - 8)
							}
							if rendered, err := renderMarkdown(note.Content, m.width); err == nil {
								m.viewContent = rendered
							} else {
								m.viewContent = "Error rendering markdown: " + err.Error()
							}
							m.status = "Updated tags for " + name
							return m, tea.ClearScreen
						}
					}
				}
			case "r":
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

		var helpParts []string
		helpParts = append(helpParts, "a:add", "d:delete", "enter:view")
		helpParts = append(helpParts, "/:search")
		if m.searchTerm != "" {
			helpParts = append(helpParts, "c:clear search")
		}
		helpParts = append(helpParts, "s:sort", "e:export", "g:toggle archived", "P:change password", "q:quit")

		helpParts = append(helpParts, "p:pin", "f:favorite", "t:tags")
		s.WriteString(helpStyle.Render(strings.Join(helpParts, "  ")))

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
