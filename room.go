package main

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RoomPublisher interface {
	publishToRoom(msg []byte)
}

type Room struct {
	dataBuffer   int
	publishLimit *rate.Limiter

	userMux sync.Mutex
	users   map[string]*User
}

func (room *Room) publishToRoom(msg []byte) {
	room.userMux.Lock()
	defer room.userMux.Unlock()

	room.publishLimit.Wait(context.Background())
	for _, user := range room.users {
		go user.conn.writeToBuffer(msg)
	}
}

func NewRoom() *Room {
	r := &Room{
		dataBuffer:   16,
		publishLimit: rate.NewLimiter(rate.Every(time.Millisecond*8), 8), // 8 tokens every 8 ms
		users:        make(map[string]*User),
	}
	return r
}
