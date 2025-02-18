package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/coder/websocket"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/message"
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

type HubServeHandler interface {
	createRoomHandler(w http.ResponseWriter, r *http.Request)
	joinRoomHandler(w http.ResponseWriter, r *http.Request)
	writeMsgToRoomHandler(w http.ResponseWriter, r *http.Request)
}

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

func (hubServe *HubServe) authorize(r *http.Request) (string, error) {
	token := r.Header.Get("Authorization")
	if len(token) <= 0 {
		return "", errors.New("Unauthorized")
	}
	return token, nil
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

	userObj := user.NewUser(token)
	err = hubServe.hub.subscribeUserToRoom(id, token, userObj)
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
		hubServe.hub.unsubscribeUserToRoom(id, token, userObj)
		hubServe.hub.deleteEmptyRoom(id)
		return
	}
	userConn := user.NewUserConn(conn)
	userObj.Connect(userConn)

	userConn.ReadMsgChannel(context.Background())
	hubServe.hub.unsubscribeUserToRoom(id, token, userObj)
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

	msg := message.Message{}
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
