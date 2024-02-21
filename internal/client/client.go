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

var USERNAME string
var ADDRESS string
var CONN *websocket.Conn
var CHAT_MSGS = []string{}

var PAGES = tview.NewPages()
var APP = tview.NewApplication()
var LOGIN = tview.NewForm()
var MAIN = tview.NewFlex()
var CHAT_AREA = tview.NewTextView()
var BUFFER_AREA = tview.NewTextArea()
var EXIT = tview.NewModal()

var BUFFER_AREA_DEFAULT_PLACEHOLDER_TEXT = "Type a message here, then press ENTER to send..."

func Start() {
	APP.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc | tcell.KeyEscape | tcell.KeyESC:
			go func() {
				APP.QueueUpdateDraw(func() {
					PAGES.SwitchToPage("exit")
				})
			}()
		default:
		}
		return event
	})

	LOGIN.AddInputField("What is your name?", "", 16, isUsernameValid, nil)
	LOGIN.AddInputField("What is the server address?", "", 32, isAddressValid, nil)
	LOGIN.AddButton("Connect", func() {
		PAGES.SwitchToPage("main")
		connect()
	})
	LOGIN.AddButton("Cancel", func() {
		PAGES.SwitchToPage("exit")
	})
	LOGIN.SetBorder(true).SetTitle("Login").SetTitleAlign(tview.AlignLeft)

	EXIT.SetText("Really quit?")
	EXIT.AddButtons([]string{"Cancel", "OK"})
	EXIT.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "OK" {
			if CONN != nil {
				disconnect()
			}
			APP.Stop()
		} else {
			if isEmpty(USERNAME) || isEmpty(ADDRESS) {
				PAGES.SwitchToPage("login")
			} else {
				PAGES.SwitchToPage("main")
			}
		}
	})

	CHAT_AREA.SetTextColor(tcell.ColorGreen)
	CHAT_AREA.SetBorder(true)
	CHAT_AREA.SetBorderStyle(tcell.StyleDefault)
	CHAT_AREA.SetTitle("Chat")
	CHAT_AREA.SetWordWrap(true)

	BUFFER_AREA.SetTitle("Send")
	BUFFER_AREA.SetTitleAlign(tview.AlignLeft)
	BUFFER_AREA.SetBorder(true)
	BUFFER_AREA.SetBorderStyle(tcell.StyleDefault)
	BUFFER_AREA.SetPlaceholder(BUFFER_AREA_DEFAULT_PLACEHOLDER_TEXT)
	BUFFER_AREA.SetWordWrap(false)
	BUFFER_AREA.SetWrap(false)
	BUFFER_AREA.SetFocusFunc(func() {
		currentText := BUFFER_AREA.GetText()
		BUFFER_AREA.SetText(strings.TrimSpace(currentText), true)
	})
	BUFFER_AREA.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			send()
		default:
		}
		return event
	})

	MAIN.SetDirection(tview.FlexRow)
	MAIN.AddItem(CHAT_AREA, 0, 4, false)
	MAIN.AddItem(BUFFER_AREA, 3, 1, true)

	PAGES.AddPage("login", LOGIN, true, true)
	PAGES.AddPage("main", MAIN, true, false)
	PAGES.AddPage("exit", EXIT, true, false)

	go listen()

	if err := APP.SetRoot(PAGES, true).SetFocus(LOGIN).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func isEmpty(s string) bool {
	return len(strings.TrimSpace(s)) < 1
}

func isUsernameValid(name string, last rune) bool {
	if isEmpty(name) {
		return false
	}
	USERNAME = name
	return true
}

func isAddressValid(address string, last rune) bool {
	if isEmpty(address) {
		return false
	}
	_, err := url.Parse(fmt.Sprintf("ws://%s", address))
	if err != nil {
		return false
	}
	ADDRESS = address
	return true
}

func connect() {
	if currPageName, _ := PAGES.GetFrontPage(); currPageName != "main" {
		return
	}

	CHAT_AREA.Clear()

	hello := fmt.Sprintf("Hello, %s.", USERNAME)
	emitToChat(hello)

	u := url.URL{Scheme: "ws", Host: ADDRESS, Path: "/ws"}
	connectingMsg := fmt.Sprintf("Connecting to %s ...", u.String())
	emitToChat(connectingMsg)

	var resp *http.Response
	var err error
	CONN, resp, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		errMsg := fmt.Sprintf("Error dialing %s: %v\n", u.String(), err.Error())
		emitToChat(errMsg)
		if resp != nil {
			errMsg = fmt.Sprintf("Handshake failed with status code %d", resp.StatusCode)
			emitToChat(errMsg)
		}
		disableBufferArea()
		return
	}

	// Send the initial hello to server
	var msg requests.Message
	msg.Username = USERNAME
	msg.Message = "hi"
	err = CONN.WriteJSON(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to handshake with server: %s", err.Error())
		emitToChat(errMsg)
		disableBufferArea()
		return
	}

	enableBufferArea()
	emitToChat("Connected!")
}

func emitToChat(msg string) {
	CHAT_MSGS = append(CHAT_MSGS, msg)
	CHAT_AREA.SetText(strings.Join(CHAT_MSGS, "\n"))
	CHAT_AREA.ScrollToEnd()
}

func send() {
	if currPageName, _ := PAGES.GetFrontPage(); currPageName != "main" {
		return
	}

	buffer := strings.TrimSpace(BUFFER_AREA.GetText())
	if buffer == "" {
		return
	}

	var msg requests.Message
	msg.Username = USERNAME
	msg.Message = buffer
	err := CONN.WriteJSON(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to send message with error: %s", err.Error())
		emitToChat(errMsg)
	}

	BUFFER_AREA.SetText("", false)
}

func listen() {
	for {
		if CONN == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		var msg requests.Message
		err := CONN.ReadJSON(&msg)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to read message with error: %s", err.Error())
			emitToChat(errMsg)
			disconnect()
			return
		}
		newChatMsg := fmt.Sprintf("%s: %s", msg.Username, msg.Message)
		emitToChat(newChatMsg)
		APP.Draw()
	}
}

func disconnect() {
	if CONN == nil {
		return
	}
	CONN.Close()
	emitToChat("Disconnected!")

	go func() {
		APP.QueueUpdateDraw(func() {
			disableBufferArea()
		})
	}()
}

func disableBufferArea() {
	BUFFER_AREA.SetDisabled(true)
	BUFFER_AREA.SetPlaceholder("")
	APP.SetFocus(CHAT_AREA)
}
func enableBufferArea() {
	BUFFER_AREA.SetDisabled(false)
	BUFFER_AREA.SetPlaceholder(BUFFER_AREA_DEFAULT_PLACEHOLDER_TEXT)
	APP.SetFocus(BUFFER_AREA)
}
