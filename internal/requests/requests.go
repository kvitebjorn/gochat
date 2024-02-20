package requests

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type Ping struct {
}
