package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/kvitebjorn/gochat/internal/requests"
)

type Client struct {
	User *requests.User
	Conn *websocket.Conn
}

var USER_COUNTER atomic.Uint64
var SERVER_USER = requests.User{UserId: 0, Username: "SERVER"}

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
var USERS = make(map[*Client]bool)
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
		usersMsg.Users = append(usersMsg.Users, *k.User)
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

	USER_COUNTER.Add(1)
	if USER_COUNTER.Load() == math.MaxUint64-1 {
		fmt.Println("Server full")
		return
	}
	user := requests.User{UserId: USER_COUNTER.Load(), Username: msg.User.Username}

	// Send back our reply, they're waiting for their user id
	var reply requests.Message
	reply.User = user
	reply.Message = "id"
	reply.Code = requests.Salutations
	err = conn.WriteJSON(&reply)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to handshake with server: %s", err.Error())
		fmt.Println(errMsg)
		return
	}

	USERS_MU.Lock()
	client := Client{&user, conn}
	BROADCAST <- msg
	USERS[&client] = true
	USERS_MU.Unlock()

	connMsg := fmt.Sprintf("%s connected!", client.User.Username)
	BROADCAST <- requests.Message{User: SERVER_USER, Message: connMsg, Code: requests.Chatter}

	// Listen for chat messages, and add them to the broadcast channel to be fanned out
	for {
		var msg requests.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			USERS_MU.Lock()
			delete(USERS, &client)
			USERS_MU.Unlock()
			disconnectMsg := fmt.Sprintf("%s disconnected!", client.User.Username)
			BROADCAST <- requests.Message{User: *client.User, Message: "bye", Code: requests.Valediction}
			BROADCAST <- requests.Message{User: SERVER_USER, Message: disconnectMsg, Code: requests.Chatter}
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
