package hub

import (
	"errors"
	"sync"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/room"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
)

type Hub struct {
	hubMu sync.Mutex
	Rooms map[string]*room.Room
	Users map[string]*user.User
}

func NewHub(rooms map[string]*room.Room, users map[string]*user.User) *Hub {
	hub := &Hub{
		Rooms: rooms,
		Users: users,
	}
	return hub
}

func (hub *Hub) DeleteEmptyRoom(roomId string) {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	if hub.Rooms[roomId].CountUsers() > 0 {
		return
	}
	delete(hub.Rooms, roomId)
}

func (hub *Hub) RegisterRoom(roomId string) (string, error) {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	if _, ok := hub.Rooms[roomId]; !ok {
		hub.Rooms[roomId] = room.NewRoom()
		return roomId, nil
	}
	return "", errors.New("Room already exists.")
}

func (hub *Hub) PublishToRoom(roomId string, msg []byte, user *user.User) error {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	roomObj, ok := hub.Rooms[roomId]
	if !ok {
		return RoomDoesNotExistsError
	}
	err := roomObj.Publish(msg, user)
	return err
}

func (hub *Hub) SubscribeUserToRoom(
	roomId string,
	user *user.User,
) error {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	roomObj, ok := hub.Rooms[roomId]
	if !ok {
		return errors.New("Room does not exist.")
	}
	roomObj.SubscribeUser(user)

	user.Room = roomId
	hub.Users[user.Username] = user

	return nil
}

func (hub *Hub) UnsubscribeUserToRoom(user *user.User) {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()
	hub.Rooms[user.Room].UnsubscribeUser(user)
	user.Disconnect()
	delete(hub.Users, user.Username)
}
