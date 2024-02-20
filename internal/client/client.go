package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gorilla/websocket"
	"github.com/kvitebjorn/gochat/internal/requests"
	"github.com/rivo/tview"
)

var username string
var conn *websocket.Conn
var chatMsgs = []string{}

var pages = tview.NewPages()
var app = tview.NewApplication()
var login = tview.NewFlex()
var flex = tview.NewFlex()
var chatArea = tview.NewTextView()
var bufferArea = tview.NewTextArea()
var exit = tview.NewModal()

func Start() {
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			if conn == nil {
				connect()
			} else {
				send()
			}
		case tcell.KeyEsc | tcell.KeyEscape | tcell.KeyESC:
			go func() {
				app.QueueUpdateDraw(func() {
					pages.SwitchToPage("exit")
				})
			}()
		default:
		}
		return event
	})

	username = "kyle"

	exit.SetTitle("Exit")
	exit.SetText("Really quit?")
	exit.AddButtons([]string{"Cancel", "OK"})
	exit.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "OK" {
			if conn != nil {
				disconnect()
			}
			app.Stop()
		} else {
			pages.SwitchToPage("main")
		}
	})

	chatArea.SetTextColor(tcell.ColorGreen)
	chatArea.SetBorder(true)
	chatArea.SetBorderStyle(tcell.StyleDefault)
	chatArea.SetText("(ENTER) to connect\n(ESC) to quit")
	chatArea.SetTitle("Chat")
	chatArea.SetWordWrap(true)

	bufferArea.SetTitle("Send")
	bufferArea.SetTitleAlign(tview.AlignLeft)
	bufferArea.SetBorder(true)
	bufferArea.SetBorderStyle(tcell.StyleDefault)
	bufferArea.SetPlaceholder("Type a message here, then press ENTER to send...")

	flex.SetDirection(tview.FlexRow)
	flex.AddItem(chatArea, 0, 4, false)
	flex.AddItem(bufferArea, 3, 1, true)

	pages.AddPage("login", login, true, false)
	pages.AddPage("main", flex, true, true)
	pages.AddPage("exit", exit, true, false)

	go listen()

	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func connect() {
	if currPageName, _ := pages.GetFrontPage(); currPageName != "main" {
		return
	}

	chatArea.Clear()

	hello := fmt.Sprintf("Hello, %s.", username)
	emitToChat(hello)

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	connectingMsg := fmt.Sprintf("Connecting to %s ...", u.String())
	emitToChat(connectingMsg)

	var resp *http.Response
	var err error
	conn, resp, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		errMsg := fmt.Sprintf("Error dialing %s: %v\n", u.String(), err.Error())
		emitToChat(errMsg)
		if resp != nil {
			errMsg = fmt.Sprintf("Handshake failed with status code %d", resp.StatusCode)
			emitToChat(errMsg)
		}
		emitToChat("Press ENTER to try again.\n")
		return
	}

	emitToChat("Connected!")
}

func emitToChat(msg string) {
	chatMsgs = append(chatMsgs, msg)
	chatArea.SetText(strings.Join(chatMsgs, "\n"))
	chatArea.ScrollToEnd()
}

func send() {
	if currPageName, _ := pages.GetFrontPage(); currPageName != "main" {
		return
	}

	buffer := strings.TrimSpace(bufferArea.GetText())
	if buffer == "" {
		return
	}

	var msg requests.Message
	msg.Username = username
	msg.Message = buffer
	err := conn.WriteJSON(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to send message with error: %s", err.Error())
		emitToChat(errMsg)
	}

	bufferArea.SetText("", true)
}

func listen() {
	for {
		if conn == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		var msg requests.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to read message with error: %s", err.Error())
			emitToChat(errMsg)
			disconnect()
			return
		}
		newChatMsg := fmt.Sprintf("%s: %s", msg.Username, msg.Message)
		emitToChat(newChatMsg)
	}
}

func disconnect() {
	if conn == nil {
		return
	}
	conn.Close()
	emitToChat("Disconnected!")
}
