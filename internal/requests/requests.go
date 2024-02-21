package requests

type StatusCode uint8

const (
	Salutations StatusCode = iota
	Valediction
	Chatter
)

type Message struct {
	Username string     `json:"username"`
	Message  string     `json:"message"`
	Code     StatusCode `json:"code"`
}

type Ping struct {
}

type UsersMsg struct {
	Users []string
}
