package main

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type userConnHandler interface {
	closeSlow()
	readMsgChannel(ctx context.Context) error
	writeMsg(ctx context.Context, msg []byte) error
}

type userConn struct {
	connMu  sync.Mutex
	closed  bool
	conn    *websocket.Conn
	msgs    chan []byte
	timeout time.Duration
}

func createUserConn(conn *websocket.Conn) *userConn {
	return &userConn{conn: conn, msgs: make(chan []byte), timeout: time.Second * 5}
}

func (uc *userConn) closeSlow() {
	uc.connMu.Lock()
	defer uc.connMu.Unlock()
	uc.closed = true
	if uc.conn != nil {
		uc.conn.Close(websocket.StatusPolicyViolation, "Connection too slow.")
	}
}

func (uc *userConn) readMsgChannel(ctx context.Context) error {
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

func (uc *userConn) writeMsg(ctx context.Context, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, uc.timeout)
	defer cancel()

	return uc.conn.Write(ctx, websocket.MessageText, msg)
}

type user struct {
	conn userConnHandler
}

func createUser(conn *websocket.Conn) *user {
	return &user{
		conn: createUserConn(conn),
	}
}
