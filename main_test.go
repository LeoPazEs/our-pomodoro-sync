package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/hub"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/room"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/serve"
	"github.com/LeoPazEs/our-pomodoro-sync/internal/pomodoro/user"
	"github.com/coder/websocket"
)

func TestCreateRoom(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]*room.Room)
	users := make(map[string]*user.User)
	hubData := hub.NewHub(rooms, users)
	hubServe := serve.NewHubServe(hubData)
	s := httptest.NewServer(hubServe)
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL+"/room/12345", "http")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	header := http.Header{}
	header.Add("Authorization", "teste")
	opts := websocket.DialOptions{HTTPHeader: header}

	c, _, err := websocket.Dial(ctx, u, &opts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	if usersCount := rooms["12345"].CountUsers(); usersCount != 1 {
		t.Fatalf("Wrong number of users registered : %d", usersCount)
	}

	userObj, ok := users["teste"]
	if !ok {
		t.Fatal("User not added to users map.")
	}
	if userObj.Room != "12345" {
		t.Fatal("Wrong user Room in user obj.")
	}
}

func TestJoinRoom(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]*room.Room)
	users := make(map[string]*user.User)
	rooms["12345"] = room.NewRoom()
	hubData := hub.NewHub(rooms, users)
	hubServe := serve.NewHubServe(hubData)
	s := httptest.NewServer(hubServe)
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL+"/room/join/12345", "http")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	header := http.Header{}
	header.Add("Authorization", "teste")
	opts := websocket.DialOptions{HTTPHeader: header}

	c, _, err := websocket.Dial(ctx, u, &opts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	if users := rooms["12345"].CountUsers(); users != 1 {
		t.Fatalf("Wrong number of users registered : %d", users)
	}
}

func TestLeaveRoom(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]*room.Room)
	rooms["12345"] = room.NewRoom()
	users := make(map[string]*user.User)
	hubData := hub.NewHub(rooms, users)
	hubServe := serve.NewHubServe(hubData)
	s := httptest.NewServer(hubServe)
	defer s.Close()
	u := "ws" + strings.TrimPrefix(s.URL+"/room/join/12345", "http")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	header := http.Header{}
	header.Add("Authorization", "teste")
	opts := websocket.DialOptions{HTTPHeader: header}
	c, _, err := websocket.Dial(ctx, u, &opts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	userObj := user.NewUser("teste")
	userConnObj := user.NewUserConn(c, ctx, cancel)
	userObj.Connect(userConnObj)

	req, err := http.NewRequest(
		http.MethodDelete,
		s.URL+"room/leave",
		nil,
	)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "teste")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 204 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Error leaving room, status code %d", res.StatusCode)
	}
	if userObj.Conn.Ctx.Err() != nil {
		t.Fatal("Context still alive.")
	}
}

func TestWriteToRoom(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]*room.Room)
	users := make(map[string]*user.User)
	rooms["12345"] = room.NewRoom()
	hubData := hub.NewHub(rooms, users)
	hubServe := serve.NewHubServe(hubData)
	s := httptest.NewServer(hubServe)
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL+"/room/join/12345", "http")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	header := http.Header{}
	header.Add("Authorization", "teste")
	opts := websocket.DialOptions{HTTPHeader: header}

	c, _, err := websocket.Dial(ctx, u, &opts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	req, err := http.NewRequest(
		http.MethodPost,
		s.URL+"/room/publish/12345",
		strings.NewReader(`{"content":"hello"}`),
	)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "teste")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 202 {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Error(string(body))
		t.Fatalf("Error sending message, status code %d", res.StatusCode)
	}

	_, msg, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != `{"content":"hello"}` {
		t.Fatal(string(msg))
	}
}

func TestWriteToRoomUnauthorized(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]*room.Room)
	users := make(map[string]*user.User)
	rooms["12345"] = room.NewRoom()
	hubData := hub.NewHub(rooms, users)
	hubServe := serve.NewHubServe(hubData)
	s := httptest.NewServer(hubServe)
	defer s.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		s.URL+"/room/publish/12345",
		strings.NewReader(`{"content":"hello"}`),
	)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "teste")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 403 {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Error(string(body))
		t.Fatalf("Error sending message, status code %d", res.StatusCode)
	}
}
