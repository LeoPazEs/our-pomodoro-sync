package serve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/coder/websocket"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/hub"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/message"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/room"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
)

type HubServe struct {
	serveMux http.ServeMux
	hub      *hub.Hub
}

func NewHubServe(hub *hub.Hub) *HubServe {
	hs := &HubServe{hub: hub}

	hs.serveMux.Handle("GET /room/{id}", JsonHandleFunc(hs.createRoomHandler))
	hs.serveMux.Handle("GET /room/join/{id}", JsonHandleFunc(hs.joinRoomHandler))
	hs.serveMux.Handle("DELETE /room/leave", JsonHandleFunc(hs.leaveRoomHandler))
	hs.serveMux.Handle("POST /room/publish/{id}", JsonHandleFunc(hs.writeMsgToRoomHandler))
	return hs
}

func (hubServe *HubServe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hubServe.serveMux.ServeHTTP(w, r)
}

func (hubServe *HubServe) authorize(r *http.Request) (*user.User, error) {
	token := r.Header.Get("Authorization")
	if len(token) <= 0 {
		return nil, errors.New("Token not found.")
	}

	userObj, ok := hubServe.hub.Users[token]
	if ok {
		return userObj, nil
	}

	return user.NewUser(token), nil
}

func (hubServe *HubServe) createRoomHandler(w http.ResponseWriter, r *http.Request) JsonError {
	_, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}

	id := r.PathValue("id")
	id, err = hubServe.hub.RegisterRoom(id)
	if err != nil {
		return NewConflictError(err, err.Error())
	}

	return hubServe.joinRoomHandler(w, r)
}

func (hubServe *HubServe) joinRoomHandler(w http.ResponseWriter, r *http.Request) JsonError {
	userObj, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}

	id := r.PathValue("id")
	err = hubServe.hub.SubscribeUserToRoom(id, userObj)
	if err != nil {
		return NewConflictError(err, err.Error())
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		http.Error(
			w,
			"Failed to establish WebSocket connection",
			http.StatusInternalServerError,
		)
		hubServe.hub.UnsubscribeUserToRoom(userObj)
		hubServe.hub.DeleteEmptyRoom(id)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userConn := user.NewUserConn(conn, ctx, cancel)
	userObj.Connect(userConn)

	defer hubServe.leaveRoomHandler(w, r)
	userConn.ReadMsgChannel(ctx)

	return nil
}

func (hubServe *HubServe) leaveRoomHandler(w http.ResponseWriter, r *http.Request) JsonError {
	userObj, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}
	hubServe.hub.UnsubscribeUserToRoom(userObj)
	hubServe.hub.DeleteEmptyRoom(userObj.Room)

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (hubServe *HubServe) writeMsgToRoomHandler(
	w http.ResponseWriter,
	r *http.Request,
) JsonError {
	err := jsonRequest(r)
	if err != nil {
		return NewBadRequestError(err, err.Error())
	}
	userObj, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}

	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	msg := message.Message{}
	err = d.Decode(&msg)
	if err != nil {
		return NewBadRequestError(err, err.Error())
	}

	jsonMsg, _ := json.Marshal(msg)
	id := r.PathValue("id")
	err = hubServe.hub.PublishToRoom(id, jsonMsg, userObj)
	if err != nil {
		if errors.Is(err, hub.RoomDoesNotExistsError) {
			return NewConflictError(err, err.Error())
		}
		if errors.Is(err, room.UserNotInRoomError) {
			return NewForbiddenError(err, err.Error())
		}
		// Error 500 return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write(jsonMsg)
	return nil
}
