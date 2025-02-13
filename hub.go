package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

type HubServe struct {
	serveMux http.ServeMux
	hub      HubRoomAndUser
}

func NewHubServe(hub HubRoomAndUser) *HubServe {
	hubServe := &HubServe{hub: hub}

	hubServe.serveMux.HandleFunc("GET /room/{id}", hubServe.createRoomHandler)
	hubServe.serveMux.HandleFunc("GET /room/join/{id}", hubServe.joinRoomHandler)
	hubServe.serveMux.HandleFunc("POST /room/publish/{id}", hubServe.writeMsgToRoomHandler)
	return hubServe
}

func (hubServe *HubServe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hubServe.serveMux.ServeHTTP(w, r)
}

type RequestHandler interface {
	authorize(r *http.Request) (string, error)
}

func (hubServe *HubServe) authorize(r *http.Request) (string, error) {
	token := r.Header.Get("Authorization")
	if len(token) <= 0 {
		return "", errors.New("Unauthorized")
	}
	return token, nil
}

type ResponseHandler interface {
	contentType(w http.ResponseWriter) http.ResponseWriter
	errorResponse(status int,
		error string,
		w http.ResponseWriter,
	) http.ResponseWriter
	successResponse(status int,
		response []byte,
		w http.ResponseWriter,
	) http.ResponseWriter
}

func (hubServe *HubServe) contentType(w http.ResponseWriter) http.ResponseWriter {
	w.Header().Set("Content-Type", "application/json")
	return w
}

func (hubServe *HubServe) errorResponse(
	status int,
	error string,
	w http.ResponseWriter,
) http.ResponseWriter {
	hubServe.contentType(w)
	w.WriteHeader(status)
	w.Write([]byte(fmt.Sprintf(`{"error": "%s"}`, error)))
	return w
}

func (hubServe *HubServe) successResponse(
	status int,
	response []byte,
	w http.ResponseWriter,
) http.ResponseWriter {
	hubServe.contentType(w)
	w.WriteHeader(status)
	w.Write(response)
	return w
}

type HubServeHandler interface {
	createRoomHandler(w http.ResponseWriter, r *http.Request)
	joinRoomHandler(w http.ResponseWriter, r *http.Request)
	writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request)
}

func (hubServe *HubServe) createRoomHandler(w http.ResponseWriter, r *http.Request) {
	_, err := hubServe.authorize(r)
	if err != nil {
		hubServe.errorResponse(http.StatusUnauthorized, err.Error(), w)
		return
	}

	id := r.PathValue("id")

	id, err = hubServe.hub.registerRoom(id)
	if err != nil {
		hubServe.errorResponse(http.StatusConflict, fmt.Sprintf(`{"error": "%s"}`, err), w)
		return
	}

	// This looks bad
	hubServe.joinRoomHandler(w, r)
}

func (hubServe *HubServe) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	token, err := hubServe.authorize(r)
	if err != nil {
		hubServe.errorResponse(http.StatusUnauthorized, err.Error(), w)
		return
	}
	id := r.PathValue("id")

	ok := hubServe.hub.checkExistsRoom(id)
	if !ok {
		hubServe.errorResponse(http.StatusConflict, "The room does not exist.", w)
		return
	}

	// Has to lock rooms
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		http.Error(
			w,
			"Failed to establish WebSocket connection",
			http.StatusInternalServerError,
		)
		return
	}

	user := NewUser(conn, token)
	hubServe.hub.subscribeUserToRoom(id, token, user)

	user.conn.readMsgChannel(context.Background())
	hubServe.hub.unsubscribeUserToRoom(id, token, user)
	hubServe.hub.deleteEmptyRoom(id)
}

func (hubServe *HubServe) writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request) {
	_, err := hubServe.authorize(r)
	if err != nil {
		hubServe.errorResponse(http.StatusUnauthorized, err.Error(), w)
		return
	}

	id := r.PathValue("id")

	if r.Header.Get("Content-Type") == "application/json" {
		msg := Message{}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body.", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		err = json.Unmarshal(body, &msg)
		if err != nil {
			hubServe.errorResponse(http.StatusBadRequest, "Decoding error, check json format.", w)
			return
		}

		ok := hubServe.hub.checkExistsRoom(id)
		if !ok {
			hubServe.errorResponse(http.StatusConflict, "The room does not exist.", w)
			return
		}

		jsonMsg, _ := json.Marshal(msg)
		hubServe.hub.publishToRoom(id, jsonMsg)
		hubServe.successResponse(http.StatusAccepted, jsonMsg, w)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{"error": "Json endpoint."}`))
}

type Hub struct {
	roomsMu sync.Mutex
	rooms   map[string]RoomUserHandler
}

func NewHub() *Hub {
	hub := &Hub{
		rooms: make(map[string]RoomUserHandler),
	}
	return hub
}

type HubRoomHandler interface {
	deleteEmptyRoom(room string)
	checkExistsRoom(room string) bool
	registerRoom(room string) (string, error)
	publishToRoom(room string, msg []byte)
}

func (hub *Hub) deleteEmptyRoom(room string) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	// For now if someone enters the room between the count and the delete the room is excluded
	if hub.rooms[room].countUsers() > 0 {
		return
	}
	delete(hub.rooms, room)
}

func (hub *Hub) checkExistsRoom(room string) bool {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	_, ok := hub.rooms[room]
	return ok
}

func (hub *Hub) registerRoom(room string) (string, error) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	if _, ok := hub.rooms[room]; !ok {
		hub.rooms[room] = NewRoom()
		return room, nil
	}
	return "", errors.New("Room already exists.")
}

func (hub *Hub) publishToRoom(room string, msg []byte) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	hub.rooms[room].publish(msg)
}

type HubUserHandler interface {
	subscribeUserToRoom(room string, id string, user UserHandler)
	unsubscribeUserToRoom(room string, username string, user UserHandler)
}

func (hub *Hub) subscribeUserToRoom(room string, username string, user UserHandler) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	hub.rooms[room].subscribeUser(username, user)
}

func (hub *Hub) unsubscribeUserToRoom(room string, username string, user UserHandler) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	hub.rooms[room].unsubscribeUser(username, user)
}

type HubRoomAndUser interface {
	HubUserHandler
	HubRoomHandler
}
