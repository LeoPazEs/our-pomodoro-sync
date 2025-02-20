package serve

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/coder/websocket"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/hub"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/message"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/room"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
)

type HubServeRequestResponseHandler interface {
	http.Handler
	RequestHandler
	ResponseHandler
	HubServeHandler
}

type RequestHandler interface {
	authorize(r *http.Request) (string, error)
}

type ResponseHandler interface {
	jsonResponse(handler http.Handler) http.Handler
	errorHandler(
		handler func(w http.ResponseWriter, r *http.Request) HubServeError,
	) http.Handler
}

type HubServeHandler interface {
	createRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError
	joinRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError
	writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError
}

type HubServe struct {
	serveMux http.ServeMux
	hub      hub.HubRoomAndUser
}

func NewHubServe(hub hub.HubRoomAndUser) *HubServe {
	hs := &HubServe{hub: hub}

	hs.serveMux.Handle(
		"GET /room/{id}",
		hs.jsonResponse(hs.errorHandler(hs.createRoomHandler)),
	)
	hs.serveMux.Handle(
		"GET /room/join/{id}",
		hs.jsonResponse(hs.errorHandler(hs.joinRoomHandler)),
	)
	hs.serveMux.Handle(
		"POST /room/publish/{id}",
		hs.jsonResponse(hs.errorHandler(hs.writeMsgToRoomHandler)),
	)
	return hs
}

func (hubServe *HubServe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hubServe.serveMux.ServeHTTP(w, r)
}

func (hubServe *HubServe) authorize(r *http.Request) (string, error) {
	token := r.Header.Get("Authorization")
	if len(token) <= 0 {
		return "", errors.New("Token not found.")
	}
	return token, nil
}

func (hubServe *HubServe) jsonResponse(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler.ServeHTTP(w, r)
	})
}

func (hubServe *HubServe) errorHandler(
	handler func(w http.ResponseWriter, r *http.Request) HubServeError,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			w.WriteHeader(err.Code())
			w.Write([]byte(err.Error()))
		}
	})
}

func (hubServe *HubServe) createRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError {
	_, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}

	id := r.PathValue("id")

	id, err = hubServe.hub.RegisterRoom(id)
	if err != nil {
		return NewConflictError(err, err.Error())
	}

	// This looks bad
	return hubServe.joinRoomHandler(w, r)
}

func (hubServe *HubServe) joinRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError {
	token, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}
	id := r.PathValue("id")

	userObj := user.NewUser(token)
	err = hubServe.hub.SubscribeUserToRoom(id, token, userObj)
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
		hubServe.hub.UnsubscribeUserToRoom(id, token, userObj)
		hubServe.hub.DeleteEmptyRoom(id)
		return nil
	}
	userConn := user.NewUserConn(conn)
	userObj.Connect(userConn)

	userConn.ReadMsgChannel(context.Background())
	hubServe.hub.UnsubscribeUserToRoom(id, token, userObj)
	hubServe.hub.DeleteEmptyRoom(id)
	return nil
}

func (hubServe *HubServe) writeMsgToRoomHandler(
	w http.ResponseWriter,
	r *http.Request,
) HubServeError {
	token, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}
	if r.Header.Get("Content-Type") != "application/json" {
		return NewBadRequestError(err, "Json endpoint.")
	}

	id := r.PathValue("id")

	msg := message.Message{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body.", http.StatusInternalServerError)
		return nil
	}
	r.Body.Close()

	err = json.Unmarshal(body, &msg)
	if err != nil {
		return NewBadRequestError(err, "Decoding error, check json format.")
	}

	jsonMsg, _ := json.Marshal(msg)
	err = hubServe.hub.PublishToRoom(id, jsonMsg, token)
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
