# Blue

A secure, local-first, terminal-based markdown note-taking application built as a Hackathon project.

## Features

- **Encrypted Storage**: Notes are encrypted locally with password protection
- **Real-time Sync**: WebSocket-based synchronization across devices
- **Rich Text Support**: Markdown rendering with syntax highlighting
- **Organization Tools**: Pin, favorite, archive, and tag your notes
- **Search & Filter**: Quick search with live filtering
- **Terminal UI**: Clean, keyboard-driven interface built with Bubble Tea

## Installation

```bash
go install github.com/electr1fy0/blue@latest
```

## Usage

Simply run `blue` to start the application. On first run, you'll create a password-protected notebook.

### Keyboard Shortcuts

#### Main List View
- `a` - Add new note
- `enter` - View selected note
- `d` - Delete note
- `/` - Search notes
- `c` - Clear search
- `s` - Toggle sort (by title/date)
- `e` - Export notes
- `g` - Toggle archived notes view
- `p` - Pin/unpin note
- `f` - Favorite/unfavorite note
- `t` - Edit tags
- `P` - Change password
- `q` - Quit

#### Note View
- `e` - Edit note
- `d` - Delete note
- `b` - Back to list
- `p` - Pin/unpin
- `f` - Favorite/unfavorite
- `t` - Edit tags
- `r` - Archive/unarchive

#### Search Mode
- `enter` - Execute search
- `esc` - Cancel search

## Note Format

Notes support YAML frontmatter for metadata:

```markdown
---
pinned: true
favorite: false
archived: false
tags: [work, project]
---

# My Note Title

Note content goes here...
```

## Sync Server

Blue includes WebSocket-based synchronization. The sync status is displayed in the bottom right:
- `connected` - Successfully connected to sync server
- `disconnected` - No connection to sync server

## Project Structure

- **Model**: TUI state management and event handling
- **Storage**: Encrypted notebook persistence
- **Server**: WebSocket synchronization
- **Utils**: Editor integration and utilities

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering
- WebSocket  for real-time sync when multiple devices have the app open


**Note**:

It has been tested on UNIX based platforms only. Experience on Windows may vary.

It's slightly buggy due to heavy reliance on LLMs.
