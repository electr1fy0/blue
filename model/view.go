package model

// func (m Model) View() string {
// 	var s strings.Builder
// 	s.WriteString(titleStyle.Render("blue — secure notes"))
// 	s.WriteString("\n\n")

// 	switch m.state {
// 	case statePass:
// 		s.WriteString("Enter password to unlock/create notebook:\n\n")
// 		s.WriteString(m.pwInput.View())
// 		s.WriteString("\n\n")
// 		s.WriteString(helpStyle.Render("ctrl+c: quit"))
// 		if m.status != "" {
// 			s.WriteString("\n")
// 			if m.lastError != "" {
// 				s.WriteString(errorStyle.Render(m.status))
// 			} else {
// 				s.WriteString(helpStyle.Render(m.status))
// 			}
// 		}

// 	case stateSearch:
// 		s.WriteString("Search notes:\n\n")
// 		s.WriteString(m.searchInput.View())
// 		s.WriteString("\n\n")
// 		s.WriteString(helpStyle.Render("enter: search  esc: cancel"))

// 	case stateConfirm:
// 		s.WriteString(warningStyle.Render(m.confirmMsg))
// 		s.WriteString("\n\n")
// 		s.WriteString(helpStyle.Render("y: confirm  n/esc: cancel"))

// 	case stateList:
// 		s.WriteString(m.list.View())
// 		s.WriteString("\n")

// 		// Build help text
// 		var helpParts []string
// 		helpParts = append(helpParts, "a:add", "d:delete", "enter:view")
// 		helpParts = append(helpParts, "/:search")
// 		if m.searchTerm != "" {
// 			helpParts = append(helpParts, "c:clear search")
// 		}
// 		helpParts = append(helpParts, "s:sort", "e:export", "g:toggle archived", "P:change password", "q:quit")
// 		// quick metadata actions
// 		helpParts = append(helpParts, "p:pin", "f:favorite", "t:tags")
// 		s.WriteString(helpStyle.Render(strings.Join(helpParts, "  ")))

// 		var statusParts []string
// 		if m.sortBy == sortByTitle {
// 			statusParts = append(statusParts, "sorted by title")
// 		} else {
// 			statusParts = append(statusParts, "sorted by date")
// 		}
// 		if m.searchTerm != "" {
// 			statusParts = append(statusParts, fmt.Sprintf("search: '%s'", m.searchTerm))
// 		}
// 		if m.showArchived {
// 			statusParts = append(statusParts, "viewing archived")
// 		}
// 		if len(statusParts) > 0 {
// 			s.WriteString("\n")
// 			s.WriteString(helpStyle.Render(strings.Join(statusParts, " • ")))
// 		}

// 		s.WriteString("\n")
// 		s.WriteString(helpStyle.Render("WS: " + m.wsStatus))

// 		if m.status != "" {
// 			s.WriteString("\n")
// 			if m.lastError != "" {
// 				s.WriteString(errorStyle.Render(m.status))
// 			} else {
// 				s.WriteString(successStyle.Render(m.status))
// 			}
// 		}

// 	case stateView:
// 		s.WriteString(titleStyle.Render(m.current))
// 		s.WriteString("\n\n")
// 		s.WriteString(m.viewContent)
// 		s.WriteString("\n\n")
// 		s.WriteString(helpStyle.Render("e:edit  d:delete  b:back  q:quit"))
// 		s.WriteString("\n")
// 		s.WriteString(helpStyle.Render("p:pin  f:favorite  t:tags  r:archive"))
// 		if m.status != "" {
// 			s.WriteString("\n")
// 			if m.lastError != "" {
// 				s.WriteString(errorStyle.Render(m.status))
// 			} else {
// 				s.WriteString(successStyle.Render(m.status))
// 			}
// 		}
// 	}

// 	return s.String()
// }
