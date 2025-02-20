package hub

import (
	"errors"
	"sync"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/room"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
)

type HubRoomHandler interface {
	DeleteEmptyRoom(roomId string)
	RegisterRoom(roomId string) (string, error)
	PublishToRoom(roomId string, msg []byte, username string) error
}

type HubUserHandler interface {
	SubscribeUserToRoom(roomId string, username string, userObj user.UserHandler) error
	UnsubscribeUserToRoom(roomId string, username string, userObj user.UserHandler)
}

type HubRoomAndUser interface {
	HubUserHandler
	HubRoomHandler
}

type Hub struct {
	roomsMu sync.Mutex
	rooms   map[string]room.RoomUserHandler
}

func NewHub() *Hub {
	hub := &Hub{
		rooms: make(map[string]room.RoomUserHandler),
	}
	return hub
}

func (hub *Hub) DeleteEmptyRoom(roomId string) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	// For now if someone enters the room between the count and the delete the room is excluded
	if hub.rooms[roomId].CountUsers() > 0 {
		return
	}
	delete(hub.rooms, roomId)
}

func (hub *Hub) RegisterRoom(roomId string) (string, error) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	if _, ok := hub.rooms[roomId]; !ok {
		hub.rooms[roomId] = room.NewRoom()
		return roomId, nil
	}
	return "", errors.New("Room already exists.")
}

func (hub *Hub) PublishToRoom(roomId string, msg []byte, username string) error {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	roomObj, ok := hub.rooms[roomId]
	if !ok {
		return RoomDoesNotExistsError
	}
	err := roomObj.Publish(msg, username)
	return err
}

func (hub *Hub) SubscribeUserToRoom(
	roomId string,
	username string,
	userObj user.UserHandler,
) error {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()

	roomObj, ok := hub.rooms[roomId]
	if !ok {
		return errors.New("Room does not exist.")
	}
	roomObj.SubscribeUser(username, userObj)
	return nil
}

func (hub *Hub) UnsubscribeUserToRoom(roomId string, username string, userObj user.UserHandler) {
	hub.roomsMu.Lock()
	defer hub.roomsMu.Unlock()
	hub.rooms[roomId].UnsubscribeUser(username)
}
