package hub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/coder/websocket"

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
	jsonResponse(
		handler func(w http.ResponseWriter, r *http.Request) HubServeError,
	) http.HandlerFunc
}

type HubServeHandler interface {
	createRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError
	joinRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError
	writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError
}

type HubServe struct {
	serveMux http.ServeMux
	hub      HubRoomAndUser
}

func NewHubServe(hub HubRoomAndUser) *HubServe {
	hubServe := &HubServe{hub: hub}

	hubServe.serveMux.HandleFunc(
		"GET /room/{id}",
		hubServe.jsonResponse(hubServe.createRoomHandler),
	)
	hubServe.serveMux.HandleFunc(
		"GET /room/join/{id}",
		hubServe.jsonResponse(hubServe.joinRoomHandler),
	)
	hubServe.serveMux.HandleFunc(
		"POST /room/publish/{id}",
		hubServe.jsonResponse(hubServe.writeMsgToRoomHandler),
	)
	return hubServe
}

func (hubServe *HubServe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hubServe.serveMux.ServeHTTP(w, r)
}

func (hubServe *HubServe) authorize(r *http.Request) (string, error) {
	token := r.Header.Get("Authorization")
	if len(token) <= 0 {
		return "", errors.New("Unauthorized")
	}
	return token, nil
}

func (hubServe *HubServe) jsonResponse(
	handler func(http.ResponseWriter, *http.Request) HubServeError,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := handler(w, r); err != nil {
			w.WriteHeader(err.Code())
			w.Write([]byte(err.Error()))
		}
	}
}

func (hubServe *HubServe) successResponse(
	status int,
	response []byte,
	w http.ResponseWriter,
) http.ResponseWriter {
	w.WriteHeader(status)
	w.Write(response)
	return w
}

func (hubServe *HubServe) createRoomHandler(w http.ResponseWriter, r *http.Request) HubServeError {
	_, err := hubServe.authorize(r)
	if err != nil {
		return NewUnauthorizedError(err, "Unauthorized")
	}

	id := r.PathValue("id")

	id, err = hubServe.hub.registerRoom(id)
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
	err = hubServe.hub.subscribeUserToRoom(id, token, userObj)
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
		hubServe.hub.unsubscribeUserToRoom(id, token, userObj)
		hubServe.hub.deleteEmptyRoom(id)
		return nil
	}
	userConn := user.NewUserConn(conn)
	userObj.Connect(userConn)

	userConn.ReadMsgChannel(context.Background())
	hubServe.hub.unsubscribeUserToRoom(id, token, userObj)
	hubServe.hub.deleteEmptyRoom(id)
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
	err = hubServe.hub.publishToRoom(id, jsonMsg, token)
	if err != nil {
		if errors.Is(err, RoomDoesNotExistsError) {
			return NewConflictError(err, err.Error())
		}
		if errors.Is(err, room.UserNotInRoomError) {
			return NewForbiddenError(err, err.Error())
		}
		// Error 500 return
	}
	hubServe.successResponse(http.StatusAccepted, jsonMsg, w)
	return nil
}
