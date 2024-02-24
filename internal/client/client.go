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

var USER requests.User
var USER_MU sync.Mutex
var HOST string
var PORT = 8080
var CONN *websocket.Conn
var CHAT_MSGS = []string{}
var CHAT_MSGS_MU sync.Mutex
var USERS = map[uint64]string{}
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
			USER_MU.Lock()
			USER.Username = maybeUsername
			USER_MU.Unlock()
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
			USER_MU.Lock()
			if isEmpty(USER.Username) || isEmpty(HOST) {
				PAGES.SwitchToPage("login")
			} else {
				PAGES.SwitchToPage("main")
			}
			USER_MU.Unlock()
		}
	})

	CHAT_AREA.SetTextColor(tcell.ColorAntiqueWhite)
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
	BUFFER_AREA.SetPlaceholderStyle(tcell.StyleDefault.Attributes(tcell.AttrDim))
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

func errorMsg(msg string) string {
	return CHAT_RED + msg + CHAT_WHITE
}

func clientMsg(msg string) string {
	return CHAT_PURPLE + msg + CHAT_WHITE
}

func serverMsg(msg string) string {
	return CHAT_GREEN + msg + CHAT_WHITE
}

func connect() {
	if currPageName, _ := PAGES.GetFrontPage(); currPageName != "main" {
		return
	}

	CHAT_AREA.Clear()

	USER_MU.Lock()
	hello := fmt.Sprintf("Hello, %s.", USER.Username)
	USER_MU.Unlock()
	emitToChat(clientMsg(hello))

	u := url.URL{Scheme: "ws", Host: HOST, Path: "/ws"}
	connectingMsg := fmt.Sprintf("Connecting to %s ...", u.String())
	emitToChat(clientMsg(connectingMsg))

	var resp *http.Response
	var err error
	CONN, resp, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		errMsg := fmt.Sprintf("Error dialing %s: %v", u.String(), err.Error())
		emitToChat(errorMsg(errMsg))
		if resp != nil {
			errMsg = fmt.Sprintf("Dialer failed with status code %d", resp.StatusCode)
			emitToChat(errorMsg(errMsg))
		}
		disableBufferArea()
		return
	}

	// Send the initial hello to server
	var msg requests.Message
	USER_MU.Lock()
	msg.User = USER
	USER_MU.Unlock()
	msg.Message = "hi"
	msg.Code = requests.Salutations
	err = CONN.WriteJSON(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to handshake with server: %s", err.Error())
		emitToChat(errorMsg(errMsg))
		disableBufferArea()
		return
	}

	// Listen for our response to get our user id
	var reply requests.Message
	err = CONN.ReadJSON(&reply)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to read message with error: %s", err.Error())
		emitToChat(errorMsg(errMsg))
		disconnect()
		return
	}
	USER_MU.Lock()
	USER.UserId = reply.User.UserId
	USER_MU.Unlock()

	enableBufferArea()
	emitToChat(clientMsg("Connected!"))
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
		emitToChat(errorMsg(errMsg))
		return
	}
	defer res.Body.Close()

	var msg requests.UsersMsg
	err = json.NewDecoder(res.Body).Decode(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Error initializing user list: %s", err)
		emitToChat(errorMsg(errMsg))
		return
	}
	USER_MU.Lock()
	userId := USER.UserId
	USER_MU.Unlock()
	for _, user := range msg.Users {
		if user.UserId == userId {
			// we already know about ourselves... this is to get users who were online before us
			continue
		}
		addToUserList(user)
	}
}

func getUserListName(user requests.User) string {
	return getUserColorTag(user.UserId) + user.Username
}

func getUserListNameSubtext(user requests.User) string {
	return fmt.Sprintf("  #%d", user.UserId)
}

func addToUserList(user requests.User) {
	USERS_MU.Lock()
	defer USERS_MU.Unlock()

	USER_LIST.AddItem(getUserListName(user), getUserListNameSubtext(user), 0, nil)
	if _, found := USERS[user.UserId]; !found {
		USERS[user.UserId] = user.Username
	}
}

func removeFromUserList(user requests.User) {
	USERS_MU.Lock()
	defer USERS_MU.Unlock()

	delete(USERS, user.UserId)
	found := USER_LIST.FindItems(user.Username, getUserListNameSubtext(user), true, false)
	USER_LIST.RemoveItem(found[0])
	APP.Draw()
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
	USER_MU.Lock()
	msg.User = USER
	USER_MU.Unlock()
	msg.Message = buffer
	msg.Code = requests.Chatter
	err := CONN.WriteJSON(&msg)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to send message with error: %s", err.Error())
		emitToChat(errorMsg(errMsg))
	}

	BUFFER_AREA.SetText("", false)
}

func listen() {
	for {
		if CONN == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		USER_MU.Lock()
		userId := USER.UserId
		USER_MU.Unlock()

		// We must wait for the handshake to complete
		// It's complete when the user id is not zero
		if userId == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		initializeUserList()
		break
	}

	for {
		var msg requests.Message
		err := CONN.ReadJSON(&msg)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to read message with error: %s", err.Error())
			emitToChat(errorMsg(errMsg))
			disconnect()
			return
		}
		switch msg.Code {
		case requests.Salutations:
			addToUserList(msg.User)
		case requests.Valediction:
			removeFromUserList(msg.User)
		case requests.Chatter:
			var newChatMsgPretty string
			switch msg.User.UserId {
			case 0:
				newChatMsg := fmt.Sprintf("%s: %s", msg.User.Username, msg.Message)
				newChatMsgPretty = serverMsg(newChatMsg)
			default:
				userPrefix := getUserColorTag(msg.User.UserId) +
					fmt.Sprintf("%s:", msg.User.Username) +
					CHAT_WHITE
				newChatMsgPretty = fmt.Sprintf("%s %s", userPrefix, msg.Message)
			}
			emitToChat(newChatMsgPretty)
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
	emitToChat(clientMsg("Disconnected!"))

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

func getUserColorTag(id uint64) string {
	userColorIdx := id % uint64(len(USER_COLOR_TAGS))
	return USER_COLOR_TAGS[userColorIdx]
}
