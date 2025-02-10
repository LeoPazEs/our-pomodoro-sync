package main

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Room struct {
	dataBuffer   int
	publishLimit *rate.Limiter

	userMux sync.Mutex
	users   map[*user]struct{}
}

func createRoom() *Room {
	r := &Room{
		dataBuffer:   16,
		publishLimit: rate.NewLimiter(rate.Every(time.Millisecond*8), 8), // 8 tokens every 8 ms
		users:        make(map[*user]struct{}),
	}
	return r
}
