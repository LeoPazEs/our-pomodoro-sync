package room

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
	"golang.org/x/time/rate"
)

type RoomUserHandler interface {
	Publish(msg []byte, username string) error
	SubscribeUser(username string, user user.UserHandler)
	UnsubscribeUser(username string)
	CountUsers() int
}

type Room struct {
	dataBuffer   int
	publishLimit *rate.Limiter

	userMux sync.Mutex
	users   map[string]user.UserHandler
}

func NewRoom() *Room {
	r := &Room{
		dataBuffer:   16,
		publishLimit: rate.NewLimiter(rate.Every(time.Millisecond*8), 8), // 8 tokens every 8 ms
		users:        make(map[string]user.UserHandler),
	}
	return r
}

func (room *Room) Publish(msg []byte, username string) error {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	if _, ok := room.users[username]; !ok {
		// Make error handling
		return errors.New("User not in room.")
	}

	room.publishLimit.Wait(context.Background())
	for _, user := range room.users {
		go user.WriteMsg(msg)
	}
	return nil
}

func (room *Room) SubscribeUser(username string, userObj user.UserHandler) {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	room.users[username] = userObj
}

func (room *Room) UnsubscribeUser(username string) {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	delete(room.users, username)
}

func (room *Room) CountUsers() int {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	return len(room.users)
}
