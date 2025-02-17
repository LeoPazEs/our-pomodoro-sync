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

	user := NewUser(token)
	err = hubServe.hub.subscribeUserToRoom(id, token, user)
	if err != nil {
		hubServe.errorResponse(http.StatusConflict, err.Error(), w)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		http.Error(
			w,
			"Failed to establish WebSocket connection",
			http.StatusInternalServerError,
		)
		hubServe.hub.unsubscribeUserToRoom(id, token, user)
		hubServe.hub.deleteEmptyRoom(id)
		return
	}
	user.conn = NewUserConn(conn)

	user.conn.readMsgChannel(context.Background())
	hubServe.hub.unsubscribeUserToRoom(id, token, user)
	hubServe.hub.deleteEmptyRoom(id)
}

func (hubServe *HubServe) writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request) {
	token, err := hubServe.authorize(r)
	if err != nil {
		hubServe.errorResponse(http.StatusUnauthorized, err.Error(), w)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		hubServe.errorResponse(http.StatusBadRequest, "Json endpoint.", w)
		return
	}

	id := r.PathValue("id")

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

	jsonMsg, _ := json.Marshal(msg)
	err = hubServe.hub.publishToRoom(id, jsonMsg, token)
	if err != nil {
		// Implement error type check
		hubServe.errorResponse(http.StatusForbidden, err.Error(), w)
		hubServe.errorResponse(http.StatusConflict, "The room does not exist.", w)
		return
	}
	hubServe.successResponse(http.StatusAccepted, jsonMsg, w)
	return
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
	registerRoom(room string) (string, error)
	publishToRoom(roomId string, msg []byte, user string) error
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

func (hub *Hub) registerRoom(room string) (string, error) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	if _, ok := hub.rooms[room]; !ok {
		hub.rooms[room] = NewRoom()
		return room, nil
	}
	return "", errors.New("Room already exists.")
}

func (hub *Hub) publishToRoom(roomId string, msg []byte, user string) error {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	room, ok := hub.rooms[roomId]
	if !ok {
		return errors.New("Room does not exist.")
	}
	err := room.publish(msg, user)
	return err
}

type HubUserHandler interface {
	subscribeUserToRoom(roomId string, username string, user UserHandler) error
	unsubscribeUserToRoom(roomId string, username string, user UserHandler)
}

func (hub *Hub) subscribeUserToRoom(roomId string, username string, user UserHandler) error {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	room, ok := hub.rooms[roomId]
	if !ok {
		return errors.New("Room does not exist.")
	}
	room.subscribeUser(username, user)
	return nil
}

func (hub *Hub) unsubscribeUserToRoom(roomId string, username string, user UserHandler) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	hub.rooms[roomId].unsubscribeUser(username, user)
}

type HubRoomAndUser interface {
	HubUserHandler
	HubRoomHandler
}
