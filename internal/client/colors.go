package client

import "fmt"

const WHITE = "#eaf1f1"
const BLACK = "#2c292d"
const RED = "#ff6188"
const ORANGE = "#fc9867"
const YELLOW = "#ffd866"
const GREEN = "#a9dc76"
const BLUE = "#78dce8"
const PURPLE = "#ab9df2"

var CHAT_WHITE = toColorTag(WHITE)
var CHAT_BLACK = toColorTag(BLACK)
var CHAT_RED = toColorTag(RED)
var CHAT_ORANGE = toColorTag(ORANGE)
var CHAT_YELLOW = toColorTag(YELLOW)
var CHAT_GREEN = toColorTag(GREEN)
var CHAT_BLUE = toColorTag(BLUE)
var CHAT_PURPLE = toColorTag(PURPLE)

func toColorTag(color string) string {
	return fmt.Sprintf("[%s]", color)
}
