package hub

import (
	"errors"
	"sync"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/room"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
)

type Hub struct {
	hubMu sync.Mutex
	rooms map[string]*room.Room
	users map[string]*user.User
}

func NewHub(rooms map[string]*room.Room, users map[string]*user.User) *Hub {
	hub := &Hub{
		rooms: rooms,
		users: users,
	}
	return hub
}

func (hub *Hub) Users(username string) (*user.User, error) {
	hub.hubMu.Lock()
	userObj, ok := hub.users[username]
	defer hub.hubMu.Unlock()
	if ok {
		return userObj, nil
	}

	return user.NewUser(username), nil
}

func (hub *Hub) RegisterRoom(roomId string) (string, error) {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	if _, ok := hub.rooms[roomId]; !ok {
		hub.rooms[roomId] = room.NewRoom()
		return roomId, nil
	}
	return "", errors.New("Room already exists.")
}

func (hub *Hub) deleteEmptyRoom(roomId string) {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	if hub.rooms[roomId].CountUsers() > 0 {
		return
	}
	delete(hub.rooms, roomId)
}

func (hub *Hub) SubscribeUserToRoom(
	roomId string,
	user *user.User,
) error {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	roomObj, ok := hub.rooms[roomId]
	if !ok {
		return errors.New("Room does not exist.")
	}
	roomObj.SubscribeUser(user)

	user.Room = roomId
	hub.users[user.Username] = user

	return nil
}

func (hub *Hub) UnsubscribeUser(userObj *user.User) {
	hub.unsubscribeUserToRoom(userObj)
	hub.deleteEmptyRoom(userObj.Room)
}

func (hub *Hub) unsubscribeUserToRoom(user *user.User) {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()
	hub.rooms[user.Room].UnsubscribeUser(user)
	user.Disconnect()
	delete(hub.users, user.Username)
}

func (hub *Hub) PublishToRoom(roomId string, msg []byte, user *user.User) error {
	hub.hubMu.Lock()
	defer hub.hubMu.Unlock()

	roomObj, ok := hub.rooms[roomId]
	if !ok {
		return RoomDoesNotExistsError
	}
	err := roomObj.Publish(msg, user)
	return err
}
