package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/hub"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/room"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/serve"
)

func main() {
	log.SetFlags(log.LstdFlags)
	err := startServer()
	if err != nil {
		log.Fatal(err)
	}
}

func startServer() error {
	if len(os.Args) < 2 {
		return errors.New("Missing address to listen in first argument!")
	}

	var hubServe serve.HubServeHandler
	rooms := make(map[string]*room.Room)
	hubServe = serve.NewHubServe(hub.NewHub(rooms))

	s := &http.Server{
		Addr:         os.Args[1],
		Handler:      hubServe,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	errc := make(chan error, 1)
	go func() {
		errc <- s.ListenAndServe()
	}()
	log.Printf("listening on %v", s.Addr)

	signs := make(chan os.Signal, 1)
	signal.Notify(signs, os.Interrupt)

	select {
	case err := <-errc:
		log.Printf("Error in serve: %v", err)
	case sig := <-signs:
		log.Printf("Terminating: %v", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return s.Shutdown(ctx)
}
