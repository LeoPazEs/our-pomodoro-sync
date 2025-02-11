package main

import (
	"context"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

type hubManager interface {
	createRoom(w http.ResponseWriter, r *http.Request)
	joinRoom(w http.ResponseWriter, r *http.Request)
	subscribeUser(room string, user *user)
	unsubscribeUser(room string, user *user)
	checkDeleteEmptyRoom(room string)
}

type Hub struct {
	serveMux http.ServeMux

	roomsMu sync.Mutex
	rooms   map[string]*Room
}

func (hub *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hub.serveMux.ServeHTTP(w, r)
}

func (hub *Hub) createRoomHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	hub.roomsMu.Lock()
	_, ok := hub.rooms[id]
	if !ok {
		hub.rooms[id] = createRoom()
	}
	hub.roomsMu.Unlock()

	if !ok {
		hub.joinRoomHandler(w, r)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": "Room already exists."}`))
	}
}

func (hub *Hub) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	hub.roomsMu.Lock()
	_, ok := hub.rooms[id]
	hub.roomsMu.Unlock()
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": "The room does not exist."}`))
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

	user.conn.readMsgChannel(context.Background())
	hub.unsubscribeUser(id, user)
	hub.checkDeleteEmptyRoom(id)
}

func (hub *Hub) subscribeUser(room string, user *user) {
	hub.rooms[room].userMux.Lock()
	defer hub.rooms[room].userMux.Unlock()
	hub.rooms[room].users[user] = struct{}{}
}

func (hub *Hub) unsubscribeUser(room string, user *user) {
	hub.rooms[room].userMux.Lock()
	defer hub.rooms[room].userMux.Unlock()
	delete(hub.rooms[room].users, user)
}

func (hub *Hub) checkDeleteEmptyRoom(room string) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	hub.rooms[room].userMux.Lock()
	if len(hub.rooms[room].users) > 0 {
		hub.rooms[room].userMux.Unlock()
		return
	}
	hub.rooms[room].userMux.Unlock()
	delete(hub.rooms, room)
}

func newHub() *Hub {
	hub := &Hub{
		rooms: make(map[string]*Room),
	}

	hub.serveMux.HandleFunc("GET /room/{id}", hub.createRoomHandler)
	hub.serveMux.HandleFunc("GET /room/join/{id}", hub.joinRoomHandler)
	// hub.serveMux.HandleFunc("GET /room/publish/{id}", hub.joinRoomHa)
	return hub
}
