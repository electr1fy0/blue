package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
)

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
