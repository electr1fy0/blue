package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Note struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WSMessage struct {
	Type string `json:"type"` // "sync", "add", "edit", "delete"
	Note *Note  `json:"note,omitempty"`
}

type Hub struct {
	clients    map[*websocket.Conn]bool
	notes      map[string]Note
	mu         sync.Mutex
	broadcast  chan WSMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		notes:      make(map[string]Note),
		broadcast:  make(chan WSMessage),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) run() {
	for {
		select {
		case conn := <-h.register:
			h.clients[conn] = true
			log.Println("Client registered")
			h.mu.Lock()
			fullSync := make([]Note, 0, len(h.notes))
			for _, note := range h.notes {
				fullSync = append(fullSync, note)
			}
			h.mu.Unlock()
			msg := WSMessage{
				Type: "full_sync",
			}
			data, _ := json.Marshal(fullSync)

			conn.WriteMessage(websocket.TextMessage, data)
			_ = conn.WriteJSON(msg)
		case conn := <-h.unregister:
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
				log.Println("Client unregistered")
			}
		case message := <-h.broadcast:
			h.mu.Lock()
			switch message.Type {
			case "add", "edit":
				current, exists := h.notes[message.Note.Title]
				if !exists || message.Note.UpdatedAt.After(current.UpdatedAt) {
					h.notes[message.Note.Title] = *message.Note
				}
			case "delete":
				delete(h.notes, message.Note.Title)
			}
			h.mu.Unlock()

			for c := range h.clients {
				err := c.WriteJSON(message)
				if err != nil {
					log.Println("Error broadcasting:", err)
					c.Close()
					delete(h.clients, c)
				}
			}
		}
	}
}

func wsHandler(h *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	h.register <- conn

	defer func() {
		h.unregister <- conn
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			log.Println("Invalid message:", err)
			continue
		}

		if wsMsg.Type == "add" || wsMsg.Type == "edit" {
			wsMsg.Note.UpdatedAt = time.Now()
		}

		h.broadcast <- wsMsg
	}
}

func main() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(hub, w, r)
	})

	log.Println("Collaboration server listening on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
