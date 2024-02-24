package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gorilla/websocket"
	"github.com/kvitebjorn/gochat/internal/requests"
	"github.com/rivo/tview"
)

var USERNAME string
var HOST string
var PORT = 8080
var CONN *websocket.Conn
var CHAT_MSGS = []string{}
var CHAT_MSGS_MU sync.Mutex
var USERS = map[string]bool{}
var USERS_MU sync.Mutex
var SERVICE_DISCOVERY = ServiceDiscovery{}

var PAGES = tview.NewPages()
var APP = tview.NewApplication()
var LOGIN = tview.NewForm()
var MAIN = tview.NewFlex()
var CHAT_AREA = tview.NewTextView()
var BUFFER_AREA = tview.NewTextArea()
var USER_LIST = tview.NewList()
var EXIT = tview.NewModal()

var BUFFER_AREA_DEFAULT_PLACEHOLDER_TEXT = "Press ENTER to send..."

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

	LOGIN.AddInputField("What is your name?", "", 16, nil, nil)
	LOGIN.AddDropDown("0 hosts detected", []string{}, -1, nil)
	go func() {
		hosts := SERVICE_DISCOVERY.Scan()
		if len(hosts) < 1 {
			return
		}
		LOGIN.RemoveFormItem(1)
		label := fmt.Sprintf("%d hosts detected", len(hosts))
		LOGIN.AddDropDown(label, hosts, 0, func(host string, optionIdx int) {
			HOST = fmt.Sprintf("%s:%d", host, PORT)
		})
		LOGIN.AddButton("Connect", func() {
			usernameField := LOGIN.GetFormItemByLabel("What is your name?")
			maybeUsername := usernameField.(*tview.InputField).GetText()
			fmt.Println(maybeUsername)
			if !isUsernameValid(maybeUsername) {
				go func() {
					APP.QueueUpdateDraw(func() {
						usernameField.(*tview.InputField).SetText("")
						APP.SetFocus(LOGIN)
					})
				}()
				return
			}
			USERNAME = maybeUsername
			PAGES.SwitchToPage("main")
			connect()
		})
		APP.Draw()
	}()
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
			if isEmpty(USERNAME) || isEmpty(HOST) {
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
	CHAT_AREA.SetMaxLines(1024)
	CHAT_AREA.SetDynamicColors(true)
	CHAT_AREA.SetToggleHighlights(true)

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

	USER_LIST.SetTitle("Users")
	USER_LIST.SetBorder(true)
	USER_LIST.SetSelectedFocusOnly(true)
	mainTopFlex := tview.NewFlex()
	mainTopFlex.SetDirection(tview.FlexColumn)
	mainTopFlex.AddItem(CHAT_AREA, 0, 3, true)
	mainTopFlex.AddItem(USER_LIST, 0, 1, true)
	MAIN.SetDirection(tview.FlexRow)
	MAIN.AddItem(mainTopFlex, 0, 4, false)
	MAIN.AddItem(BUFFER_AREA, 3, 1, true)

	PAGES.AddPage("login", LOGIN, true, true)
	PAGES.AddPage("main", MAIN, true, false)
	PAGES.AddPage("exit", EXIT, true, false)

	go listen()

	if err := APP.SetRoot(PAGES, true).
		SetFocus(LOGIN).
		EnablePaste(true).
		EnableMouse(true).
		Run(); err != nil {
		panic(err)
	}
}

func isEmpty(s string) bool {
	return len(strings.TrimSpace(s)) < 1
}

func isUsernameValid(name string) bool {
	if isEmpty(name) || strings.ToUpper(name) == "SERVER" {
		return false
	}
	return true
}

func connect() {
	if currPageName, _ := PAGES.GetFrontPage(); currPageName != "main" {
		return
	}

	CHAT_AREA.Clear()

	hello := fmt.Sprintf("Hello, %s.", USERNAME)
	emitToChat(hello)

	u := url.URL{Scheme: "ws", Host: HOST, Path: "/ws"}
	connectingMsg := fmt.Sprintf("Connecting to %s ...", u.String())
	emitToChat(connectingMsg)

	var resp *http.Response
	var err error
	CONN, resp, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		errMsg := fmt.Sprintf("Error dialing %s: %v", u.String(), err.Error())
		emitToChat(errMsg)
		if resp != nil {
			errMsg = fmt.Sprintf("Dialer failed with status code %d", resp.StatusCode)
			emitToChat(errMsg)
		}
		disableBufferArea()
		return
	}

	// Send the initial hello to server
	var msg requests.Message
	msg.Username = USERNAME
	msg.Message = "hi"
	msg.Code = requests.Salutations
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
	CHAT_MSGS_MU.Lock()
	defer CHAT_MSGS_MU.Unlock()

	if len(CHAT_MSGS) > 2048 {
		CHAT_MSGS = CHAT_MSGS[1024:]
	}

	CHAT_MSGS = append(CHAT_MSGS, msg)
	CHAT_AREA.SetText(strings.Join(CHAT_MSGS, "\n"))
	CHAT_AREA.ScrollToEnd()
}

func initializeUserList() {
	requestURL := fmt.Sprintf("http://%s/users", HOST)
	var myClient = &http.Client{Timeout: 10 * time.Second}
	res, err := myClient.Get(requestURL)
	if err != nil || res.StatusCode != 200 {
		errMsg := fmt.Sprintf("Error initializing user list: %s %v", err, res.StatusCode)
		emitToChat(errMsg)
		return
	}
	defer res.Body.Close()

	var msg requests.UsersMsg
	err = json.NewDecoder(res.Body).Decode(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Error initializing user list: %s", err)
		emitToChat(errMsg)
		return
	}
	for _, user := range msg.Users {
		// TODO: won't work for duplicate usernames...
		//       probably should an add `id` to each msg request too
		if user == USERNAME {
			// we would've already known about ourselves...
			// this is to get users who were online before us
			continue
		}
		addToUserList(user)
	}
}

func addToUserList(user string) {
	USER_LIST.AddItem(user, "online", 0, nil)
	USERS_MU.Lock()
	USERS[user] = true
	USERS_MU.Unlock()
}

func removeFromUserList(user string) {
	USER_LIST.Clear()
	USERS_MU.Lock()
	delete(USERS, user)
	for user, online := range USERS {
		if !online {
			continue
		}
		USER_LIST.AddItem(user, "online", 0, nil)
	}
	USERS_MU.Unlock()
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
	msg.Code = requests.Chatter
	err := CONN.WriteJSON(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to send message with error: %s", err.Error())
		emitToChat(errMsg)
	}

	BUFFER_AREA.SetText("", false)
}

func listen() {
	isUserListInitialized := false
	for {
		if CONN == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if !isUserListInitialized {
			initializeUserList()
			isUserListInitialized = true
		}

		var msg requests.Message
		err := CONN.ReadJSON(&msg)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to read message with error: %s", err.Error())
			emitToChat(errMsg)
			disconnect()
			return
		}
		switch msg.Code {
		case requests.Salutations:
			addToUserList(msg.Username)
		case requests.Valediction:
			removeFromUserList(msg.Username)
		case requests.Chatter:
			newChatMsg := fmt.Sprintf("%s: %s", msg.Username, msg.Message)
			emitToChat(newChatMsg)
		default:
		}
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
