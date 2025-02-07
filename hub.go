package main

import (
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
	hub.rooms[id] = &Room{}
	hub.roomsMu.Unlock()
}

func newHub() *Hub {
	hub := &Hub{
		rooms: make(map[string]*Room),
	}

	hub.serveMux.HandleFunc("/room/{id}", hub.createRoom)
	return hub
}
