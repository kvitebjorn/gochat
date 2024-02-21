package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/kvitebjorn/gochat/internal/requests"
)

type Client struct {
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

var clientsMu sync.Mutex
var clients = make(map[Client]bool)
var broadcast = make(chan requests.Message)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the Chat Room!")
}

func pong(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pong!")
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	clientsCopy := make([]string, len(clients))
	clientsMu.Lock()
	for k := range clients {
		clientsCopy = append(clientsCopy, k.Username)
	}
	clientsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientsCopy)
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
	if err != nil {
		return
	}
	client := Client{msg.Username, conn}

	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()

	connMsg := fmt.Sprintf("%s connected!", client.Username)
	broadcast <- requests.Message{Username: "SERVER", Message: connMsg}

	// Listen for chat messages, and add them to the broadcast channel to be fanned out
	for {
		var msg requests.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			clientsMu.Lock()
			delete(clients, client)
			clientsMu.Unlock()
			disconnectMsg := fmt.Sprintf("%s disconnected!", client.Username)
			broadcast <- requests.Message{Username: "SERVER", Message: disconnectMsg}
			return
		}

		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast

		clientsMu.Lock()
		for client := range clients {
			err := client.Conn.WriteJSON(msg)
			if err != nil {
				fmt.Println(err)
			}
		}
		clientsMu.Unlock()
	}
}
