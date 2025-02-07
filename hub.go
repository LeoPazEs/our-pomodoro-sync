package main

import (
	"fmt"
	"net/http"
	"sync"
)

type roomManager interface {
	createRoom(w http.ResponseWriter, r *http.Request)
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
		hub.rooms[id] = &Room{}
	}
	hub.roomsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(fmt.Sprintf(`{"id": "%s"}`, id)))
	} else {
		http.Error(w, `{"error": "Room already exists."}`, http.StatusConflict)
	}
}

func newHub() *Hub {
	hub := &Hub{
		rooms: make(map[string]*Room),
	}

	hub.serveMux.HandleFunc("POST /room/{id}", hub.createRoom)
	return hub
}
