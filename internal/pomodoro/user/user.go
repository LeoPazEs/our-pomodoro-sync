package user

type User struct {
	conn     ConnHandler
	Username string
	Room     string
}

func NewUser(token string) *User {
	return &User{
		Username: token,
	}
}

func (user *User) WriteMsg(msg []byte) {
	user.conn.writeToBuffer(msg)
}

func (user *User) Connect(ws ConnHandler) {
	user.conn = ws
}
