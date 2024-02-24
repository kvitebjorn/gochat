package requests

type StatusCode uint8

const (
	Salutations StatusCode = iota
	Valediction
	Chatter
)

type User struct {
	UserId   uint64
	Username string
}

type Message struct {
	User    User       `json:"user"`
	Message string     `json:"message"`
	Code    StatusCode `json:"code"`
}

type Ping struct {
}

type UsersMsg struct {
	Users []User
}
