package client

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/gorilla/websocket"
	"github.com/kvitebjorn/gochat/internal/requests"
)

func Start() {
	s, err := tcell.NewScreen()
	if err != nil {
		//log.Fatalf("%+v", err) TODO use `log`
		fmt.Println("tcell new screen fatal")
		return
	}
	if err := s.Init(); err != nil {
		//log.Fatalf("%+v", err)
		fmt.Println("tcell init fatal")
		return
	}

	// Set default text style
	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	s.SetStyle(defStyle)
	s.EnablePaste()

	// Clear screen
	s.Clear()

	// Begin showing the terminal screen
	s.Show()

	// Set up panic and trace
	quit := func() {
		maybePanic := recover()
		s.Fini()
		if maybePanic != nil {
			panic(maybePanic)
		}
	}
	defer quit()

	xmax, ymax := s.Size()
	var currx, curry int

	drawText(s, currx, curry, xmax-1, ymax-2, tcell.StyleDefault, "Starting client...", true)
	curry++
	s.Show()

	// Websocket stuff
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	drawText(s, currx, curry, xmax-1, ymax-2, tcell.StyleDefault, fmt.Sprintf("Connecting to %s\n", u.String()), true)
	curry++
	s.Show()

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Printf("dial: %v\n", err)
		if resp != nil {
			fmt.Printf("handshake failed with status %d\n", resp.StatusCode)
		}
	}
	defer conn.Close()
	drawText(s, currx, curry, xmax-1, ymax-2, tcell.StyleDefault, "Connected!", true)
	curry++
	s.Show()

	visibileCursorX := 1
	visibleCursorY := ymax - 1
	virtualCursorX := 1
	msgLimitX := xmax - 1
	var sb strings.Builder

	// UI Event loop
	for {
		s.ShowCursor(visibileCursorX, visibleCursorY)

		// Update screen
		s.Show()

		// Poll event
		ev := s.PollEvent()

		// Process event
		switch ev := ev.(type) {
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return
			} else if ev.Key() == tcell.KeyCtrlL {
				s.Sync()
			} else if ev.Key() == tcell.KeyEnter {
				if strings.TrimSpace(sb.String()) == "" {
					break
				}
				var msg requests.Message
				msg.Username = "kyle"
				msg.Message = sb.String()
				added := drawText(s, currx, curry, xmax-1, ymax-2, tcell.StyleDefault, fmt.Sprintf("%s: %s", msg.Username, msg.Message), true)
				curry += added + 1
				err := conn.WriteJSON(&msg)
				if err != nil {
					fmt.Println(err)
					break
				}
				sb.Reset()
				visibileCursorX = 1
				virtualCursorX = 1
				drawText(s, visibileCursorX, visibleCursorY, xmax-1, ymax, tcell.StyleDefault, strings.Repeat(" ", xmax), false)
			} else if ev.Key() == tcell.KeyBackspace ||
				ev.Key() == tcell.KeyBackspace2 ||
				ev.Key() == tcell.KeyDelete {
				if sb.Len() < 1 {
					break
				}
				visibileCursorX--
				virtualCursorX--
				drawText(s, visibileCursorX, visibleCursorY, xmax, ymax, tcell.StyleDefault, " ", false)
				currentString := sb.String()
				sb.Reset()
				sb.WriteString(currentString[0 : len(currentString)-1])
			} else {
				if !unicode.IsPrint(ev.Rune()) {
					break
				}
				sb.WriteRune(ev.Rune())
				if visibileCursorX >= msgLimitX {
					diff := virtualCursorX - msgLimitX
					drawText(s, 1, visibleCursorY, xmax-1, ymax, tcell.StyleDefault, sb.String()[diff:], false)
				} else {
					drawText(s, visibileCursorX, visibleCursorY, xmax-1, ymax, tcell.StyleDefault, string(ev.Rune()), false)
					visibileCursorX++
				}
				virtualCursorX++
			}
		}
	}
}

func drawText(s tcell.Screen, x1, y1, x2, y2 int, style tcell.Style, text string, wrap bool) int {
	row := y1
	col := x1
	added := 0
	for _, r := range text {
		s.SetContent(col, row, r, nil, style)
		col++
		if col >= x2 && wrap {
			row++
			added++
			col = x1
		}
		if row > y2 {
			break
		}
	}

	return added
}
