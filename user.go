package main

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type UserConnHandler interface {
	closeSlow()
	readMsgChannel(ctx context.Context) error
	writeMsg(ctx context.Context, msg []byte) error
}

type UserConn struct {
	connMu  sync.Mutex
	closed  bool
	conn    *websocket.Conn
	msgs    chan []byte
	timeout time.Duration
}

func createUserConn(conn *websocket.Conn) *UserConn {
	return &UserConn{conn: conn, msgs: make(chan []byte), timeout: time.Second * 5}
}

func (uc *UserConn) closeSlow() {
	uc.connMu.Lock()
	defer uc.connMu.Unlock()
	uc.closed = true
	if uc.conn != nil {
		uc.conn.Close(websocket.StatusPolicyViolation, "Connection too slow.")
	}
}

func (uc *UserConn) readMsgChannel(ctx context.Context) error {
	uc.connMu.Lock()
	if uc.closed {
		uc.connMu.Unlock()
		return net.ErrClosed
	}
	uc.connMu.Unlock()

	defer uc.conn.CloseNow()
	ctx = uc.conn.CloseRead(ctx)
	for {
		select {
		case msg := <-uc.msgs:
			err := uc.writeMsg(ctx, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (uc *UserConn) writeMsg(ctx context.Context, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, uc.timeout)
	defer cancel()

	return uc.conn.Write(ctx, websocket.MessageText, msg)
}

type User struct {
	conn  UserConnHandler
	token string
}

func createUser(conn *websocket.Conn, token string) *User {
	return &User{
		conn:  createUserConn(conn),
		token: token,
	}
}
