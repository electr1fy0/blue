package server

import "github.com/gorilla/websocket"

type WsConnected struct {
	Conn *websocket.Conn
}
type WsMessage struct {
	Data []byte
}
type WsError struct {
	Err error
}
