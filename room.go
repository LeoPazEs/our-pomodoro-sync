package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Room struct {
	dataBuffer   int
	publishLimit *rate.Limiter

	userMux sync.Mutex
	users   map[string]UserHandler
}

func NewRoom() *Room {
	r := &Room{
		dataBuffer:   16,
		publishLimit: rate.NewLimiter(rate.Every(time.Millisecond*8), 8), // 8 tokens every 8 ms
		users:        make(map[string]UserHandler),
	}
	return r
}

type RoomUserHandler interface {
	publish(msg []byte, user string) error
	subscribeUser(username string, user UserHandler)
	unsubscribeUser(username string, user UserHandler)
	countUsers() int
}

func (room *Room) publish(msg []byte, user string) error {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	if _, ok := room.users[user]; !ok {
		// Make error handling
		return errors.New("User not in room.")
	}

	room.publishLimit.Wait(context.Background())
	for _, user := range room.users {
		go user.writeMsg(msg)
	}
	return nil
}

func (room *Room) subscribeUser(username string, user UserHandler) {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	room.users[username] = user
}

func (room *Room) unsubscribeUser(username string, user UserHandler) {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	delete(room.users, username)
}

func (room *Room) countUsers() int {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	return len(room.users)
}
