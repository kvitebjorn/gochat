package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/kvitebjorn/gochat/internal/requests"
)

type User struct {
	Username string
	Conn     *websocket.Conn
}

func Start() {
	http.HandleFunc("/", home)
	http.HandleFunc("/ping", pong)
	http.HandleFunc("/ws", handleConnection)
	http.HandleFunc("/users", handleUsers)

	go handleMessages()

	fmt.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("Error starting server: " + err.Error())
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var USERS_MU sync.Mutex
var USERS = make(map[User]bool)
var BROADCAST = make(chan requests.Message)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the Chat Room!")
}

func pong(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pong!")
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	var usersMsg requests.UsersMsg
	USERS_MU.Lock()
	for k := range USERS {
		usersMsg.Users = append(usersMsg.Users, k.Username)
	}
	USERS_MU.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&usersMsg)
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	// Wait for initial hello message
	var msg requests.Message
	err = conn.ReadJSON(&msg)
	if err != nil || msg.Code != requests.Salutations {
		fmt.Printf("%v %s\n", msg.Code, err.Error())
		return
	}
	user := User{msg.Username, conn}
	BROADCAST <- msg

	USERS_MU.Lock()
	USERS[user] = true
	USERS_MU.Unlock()

	connMsg := fmt.Sprintf("%s connected!", user.Username)
	BROADCAST <- requests.Message{Username: "SERVER", Message: connMsg, Code: requests.Chatter}

	// Listen for chat messages, and add them to the broadcast channel to be fanned out
	for {
		var msg requests.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			USERS_MU.Lock()
			delete(USERS, user)
			USERS_MU.Unlock()
			disconnectMsg := fmt.Sprintf("%s disconnected!", user.Username)
			BROADCAST <- requests.Message{Username: user.Username, Message: "bye", Code: requests.Valediction}
			BROADCAST <- requests.Message{Username: "SERVER", Message: disconnectMsg, Code: requests.Chatter}
			return
		}

		BROADCAST <- msg
	}
}

func handleMessages() {
	for {
		msg := <-BROADCAST

		USERS_MU.Lock()
		for user := range USERS {
			err := user.Conn.WriteJSON(msg)
			if err != nil {
				fmt.Println(err)
			}
		}
		USERS_MU.Unlock()
	}
}
