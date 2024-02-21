package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/kvitebjorn/gochat/internal/requests"
)

func Start() {
	http.HandleFunc("/", home)
	http.HandleFunc("/ping", pong)
	http.HandleFunc("/ws", handleConnection)

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
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan requests.Message)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the Chat Room!")
}

func pong(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pong!")
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	broadcast <- requests.Message{Username: "SERVER", Message: "Client connected!"}

	for {
		var msg requests.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			clientsMu.Lock()
			delete(clients, conn)
			clientsMu.Unlock()
			broadcast <- requests.Message{Username: "SERVER", Message: "Client disconnected!"}
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
			err := client.WriteJSON(msg)
			if err != nil {
				fmt.Println(err)
			}
		}
		clientsMu.Unlock()
	}
}
