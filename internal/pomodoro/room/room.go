package room

import (
	"context"
	"sync"
	"time"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
	"golang.org/x/time/rate"
)

type Room struct {
	dataBuffer   int
	publishLimit *rate.Limiter

	userMux sync.Mutex
	users   map[string]*user.User
}

func NewRoom() *Room {
	r := &Room{
		dataBuffer:   16,
		publishLimit: rate.NewLimiter(rate.Every(time.Millisecond*8), 8), // 8 tokens every 8 ms
		users:        make(map[string]*user.User),
	}
	return r
}

func (room *Room) Publish(msg []byte, user *user.User) error {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	if _, ok := room.users[user.Username]; !ok {
		// Make error handling
		return UserNotInRoomError
	}

	room.publishLimit.Wait(context.Background())
	for _, user := range room.users {
		go user.WriteMsg(msg)
	}
	return nil
}

func (room *Room) SubscribeUser(user *user.User) {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	room.users[user.Username] = user
}

func (room *Room) UnsubscribeUser(user *user.User) {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	delete(room.users, user.Username)
}

func (room *Room) CountUsers() int {
	room.userMux.Lock()
	defer room.userMux.Unlock()
	return len(room.users)
}
