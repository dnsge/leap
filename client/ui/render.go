package ui

import (
	"github.com/gdamore/tcell"
	"strconv"
)

const (
	BorderHorizontal  = '─'
	BorderVertical    = '│'
	BorderTopLeft     = '╭'
	BorderTopRight    = '╮'
	BorderBottomLeft  = '╰'
	BorderBottomRight = '╯'
)

func drawBorder(s tcell.Screen, x, y, w, h int) {
	drawHorizontalLine(s, x, y, w, BorderHorizontal)
	drawHorizontalLine(s, x, y+h, w, BorderHorizontal)
	drawVerticalLine(s, x, y, h, BorderVertical)
	drawVerticalLine(s, x+w, y, h, BorderVertical)
	s.SetCell(x, y, tcell.StyleDefault, BorderTopLeft)
	s.SetCell(x+w, y, tcell.StyleDefault, BorderTopRight)
	s.SetCell(x, y+h, tcell.StyleDefault, BorderBottomLeft)
	s.SetCell(x+w, y+h, tcell.StyleDefault, BorderBottomRight)
}

func drawHorizontalLine(s tcell.Screen, x, y, w int, r rune) {
	for i := 0; i < w; i++ {
		s.SetCell(x+i, y, tcell.StyleDefault, r)
	}
}

func drawVerticalLine(s tcell.Screen, x, y, h int, r rune) {
	for i := 0; i < h; i++ {
		s.SetCell(x, y+i, tcell.StyleDefault, r)
	}
}

func drawIntStyle(s tcell.Screen, x, y, num int, style tcell.Style) {
	asString := strconv.Itoa(num)
	drawStringStyle(s, x, y, asString, style)
}

func drawInt(s tcell.Screen, x, y, num int) {
	drawIntStyle(s, x, y, num, tcell.StyleDefault)
}

func drawStringStyle(s tcell.Screen, x, y int, str string, style tcell.Style) {
	for i := 0; i < len(str); i++ {
		s.SetCell(x+i, y, style, rune(str[i]))
	}
}

func drawString(s tcell.Screen, x, y int, str string) {
	drawStringStyle(s, x, y, str, tcell.StyleDefault)
}

func drawCenteredStringStyle(s tcell.Screen, x, y int, str string, style tcell.Style) {
	centerX := x - len(str) / 2
	drawStringStyle(s, centerX, y, str, style)
}

func drawCenteredString(s tcell.Screen, x, y int, str string) {
	drawCenteredStringStyle(s, x, y, str, tcell.StyleDefault)
}

func drawStringLeft(s tcell.Screen, x, y int, str string) {
	drawString(s, x - len(str), y, str)
}