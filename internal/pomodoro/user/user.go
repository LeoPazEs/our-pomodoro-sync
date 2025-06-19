package user

import (
	"errors"
)

type User struct {
	Conn     *UserConn
	Username string
	Room     string
}

func NewUser(token string) *User {
	return &User{
		Username: token,
	}
}

func (user *User) WriteMsg(msg []byte) {
	user.Conn.writeToBuffer(msg)
}

func (user *User) Connect(ws *UserConn) {
	user.Conn = ws
}

func (user *User) Disconnect() error {
	if user.Conn == nil {
		return errors.New("Connection does not exist to disconnect.")
	}
	user.Conn.Cancel()
	return nil
}
