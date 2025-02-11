package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

type hubManager interface {
	createRoomHandler(w http.ResponseWriter, r *http.Request)
	joinRoomHandler(w http.ResponseWriter, r *http.Request)
	writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request)
	subscribeUser(room string, username string, user *User)
	unsubscribeUser(room string, username string, user *User)
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
	token := r.Header.Get("Authorization")
	if len(token) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Unauthorized."}`))
		return
	}
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
	token := r.Header.Get("Authorization")
	if len(token) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Unauthorized."}`))
		return
	}
	id := r.PathValue("id")

	hub.roomsMu.Lock()
	_, ok := hub.rooms[id]
	hub.roomsMu.Unlock()
	if !ok {
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

	user := createUser(conn, token)
	hub.subscribeUser(id, token, user)

	user.conn.readMsgChannel(context.Background())
	hub.unsubscribeUser(id, token, user)
	hub.checkDeleteEmptyRoom(id)
}

func (hub *Hub) writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if len(token) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Unauthorized."}`))
		return
	}
	id := r.PathValue("id")

	if r.Header.Get("Content-Type") == "application/json" {
		msg := message{}
		err := json.NewDecoder(r.Body).Decode(&msg)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "Decoding error, check json format."}`))
			return
		}
		hub.roomsMu.Lock()
		if _, ok := hub.rooms[id]; !ok {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error": "The room does not exist."}`))
			return
		}

	}

	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{"error": "Json endpoint."}`))
}

func (hub *Hub) subscribeUser(room string, username string, user *User) {
	hub.rooms[room].userMux.Lock()
	defer hub.rooms[room].userMux.Unlock()
	hub.rooms[room].users[username] = user
}

func (hub *Hub) unsubscribeUser(room string, username string, user *User) {
	hub.rooms[room].userMux.Lock()
	defer hub.rooms[room].userMux.Unlock()
	delete(hub.rooms[room].users, username)
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
	hub.serveMux.HandleFunc("POST /room/publish/{id}", hub.writeMsgToRoomHandler)
	return hub
}
