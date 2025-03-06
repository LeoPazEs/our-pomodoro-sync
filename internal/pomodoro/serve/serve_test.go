package serve

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
	"github.com/coder/websocket"
)

func TestCreateRoom(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]room.RoomUserHandler)
	hubData := hub.NewHub(rooms)
	hubServe := NewHubServe(hubData)
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

	if users := rooms["12345"].CountUsers(); users != 1 {
		t.Fatalf("Wrong number of users registered : %d", users)
	}
}

func TestJoinRoom(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]room.RoomUserHandler)
	rooms["12345"] = room.NewRoom()
	hubData := hub.NewHub(rooms)
	hubServe := NewHubServe(hubData)
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

func TestWriteToRoom(t *testing.T) {
	t.Parallel()
	rooms := make(map[string]room.RoomUserHandler)
	rooms["12345"] = room.NewRoom()
	hubData := hub.NewHub(rooms)
	hubServe := NewHubServe(hubData)
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
	rooms := make(map[string]room.RoomUserHandler)
	rooms["12345"] = room.NewRoom()
	hubData := hub.NewHub(rooms)
	hubServe := NewHubServe(hubData)
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
