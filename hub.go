package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

type hubManager interface {
	createRoom(w http.ResponseWriter, r *http.Request)
	joinRoom(w http.ResponseWriter, r *http.Request)
	subscribeUser()
	unsubscribeUser()
}

type Hub struct {
	serveMux http.ServeMux

	roomsMu sync.Mutex
	rooms   map[string]*Room
}

func (hub *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hub.serveMux.ServeHTTP(w, r)
}

func (hub *Hub) createRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	hub.roomsMu.Lock()
	_, ok := hub.rooms[id]
	if !ok {
		hub.rooms[id] = createRoom()
	}
	hub.roomsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if !ok {
		hub.joinRoom(w, r)
	} else {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": "Room already exists."}`))
	}
}

func (hub *Hub) joinRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	_, ok := hub.rooms[id]
	if !ok {
		http.Error(w, `{"error": "The room does not exist."}`, http.StatusConflict)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		http.Error(
			w,
			"Failed to establish WebSocket connection",
			http.StatusInternalServerError,
		)
		return
	}

	user := createUser(conn)

	hub.subscribeUser(id, user)

	go user.conn.readMsgChannel(context.Background())

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf(`{"id": "%s"}`, id)))
}

func (hub *Hub) subscribeUser(room string, user *user) {
	hub.rooms[room].userMux.Lock()
	defer hub.rooms[room].userMux.Unlock()
	// Remember to add a close and start another connection is user already in room
	hub.rooms[room].users[user] = struct{}{}
}

func (hub *Hub) unsubscribeUser(room string, user *user) {
	hub.rooms[room].userMux.Lock()
	defer hub.rooms[room].userMux.Unlock()
	// Remeber to check if still has users in the room to close it if no one is there
	delete(hub.rooms[room].users, user)
}

func newHub() *Hub {
	hub := &Hub{
		rooms: make(map[string]*Room),
	}

	hub.serveMux.HandleFunc("GET /room/{id}", hub.createRoom)
	return hub
}
