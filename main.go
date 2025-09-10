package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/electr1fy0/blue/model"
	"github.com/electr1fy0/blue/server"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

func main() {
	// Check if we have a proper terminal using golang.org/x/term
	if !isatty() {
		fmt.Fprintf(os.Stderr, "This program requires a terminal\n")
		os.Exit(1)
	}

	p := tea.NewProgram(
		model.InitialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Start background websocket client goroutine.
	// It will send messages back to the Bubble Tea program via p.Send.
	go func() {
		// Add a small delay to ensure the TUI is fully initialized
		time.Sleep(100 * time.Millisecond)

		// server URL: env override or default
		wsURL := os.Getenv("BLUE_SERVER_URL")
		if wsURL == "" {
			wsURL = "ws://localhost:8080/ws"
		}

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			p.Send(server.WsError{Err: err})
			return
		}
		// send connected message with conn
		p.Send(server.WsConnected{Conn: conn})

		// read loop
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				p.Send(server.WsError{Err: err})
				_ = conn.Close()
				return
			}
			p.Send(server.WsMessage{Data: msg})
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func isatty() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
