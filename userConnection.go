package main

import "github.com/coder/websocket"

type UserConnection struct {
	socket  *websocket.Conn
	recieve chan []byte
	room    *Room
}
