package user

type UserHandler interface {
	WriteMsg([]byte)
	Connect(ws ConnHandler)
}

type User struct {
	conn     ConnHandler
	username string
}

func NewUser(token string) *User {
	return &User{
		username: token,
	}
}

func (user *User) WriteMsg(msg []byte) {
	user.conn.writeToBuffer(msg)
}

func (user *User) Connect(ws ConnHandler) {
	user.conn = ws
}
