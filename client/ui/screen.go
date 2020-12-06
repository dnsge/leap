package ui

import (
	"context"
	"fmt"
	"github.com/dnsge/leap/client"
	"github.com/gdamore/tcell"
	"math"
	"net/http"
	"os"
	"time"
)

const updateInterval = time.Millisecond * 100

func makeScreen() tcell.Screen {
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create graphical user interface: %v\n", err)
		os.Exit(1)
	}

	if err = s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot initialize graphical user interface: %v\n", err)
		os.Exit(1)
	}

	s.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite))

	return s
}

type LeapUI struct {
	client *client.LeapClient
	screen tcell.Screen

	ctx        context.Context
	cancelFunc context.CancelFunc

	status       string
	requestCount int
}

func New(c *client.LeapClient, ctx context.Context, cancelFunc context.CancelFunc) *LeapUI {
	u := &LeapUI{
		client: c,
		screen: makeScreen(),

		ctx:          ctx,
		cancelFunc:   cancelFunc,
		status:       "",
		requestCount: 0,
	}

	c.OnStateChange = func(state client.State) {
		u.status = state.String()
		if state == client.Disconnected {
			time.Sleep(time.Second * 2)
			cancelFunc()
		}
	}

	c.OnRequest = func(*http.Request) {
		u.requestCount++
	}

	return u
}

func (u *LeapUI) Run() {
	u.screen.Clear()

	go u.pollEvents()
	go func() {
		for {
			u.drawScreen()
			time.Sleep(updateInterval)
		}
	}()

	<-u.ctx.Done()
	u.screen.Clear()
	u.screen.Fini()
}

func (u *LeapUI) formatProxyString() string {
	s := ""
	if u.client.Config.Secure {
		s = "s"
	}

	return fmt.Sprintf("http%s://%s.%s --> http://127.0.0.1:%d", s, u.client.Subdomain(), u.client.Config.Domain, u.client.Config.LocalPort)
}

func (u *LeapUI) drawScreen() {
	u.screen.Clear()
	w, h := u.screen.Size()

	if w == 0 || h == 0 {
		return
	}

	boxW := int(math.Min(float64(w), 75)) - 1
	boxH := int(math.Min(float64(h), 11)) - 1

	boxX := (w - boxW) / 2
	boxY := (h - boxH) / 2

	boxCenterX := boxX + boxW/2
	//boxCenterY := boxY + boxH/2

	drawBorder(u.screen, boxX, boxY, boxW, boxH)
	drawCenteredString(u.screen, boxCenterX, boxY+1, "-- Leap Client --")
	drawCenteredString(u.screen, boxCenterX, boxY+4, fmt.Sprintf("Status: %s", u.status))
	drawCenteredStringStyle(u.screen, boxCenterX, boxY+5, u.formatProxyString(), tcell.StyleDefault.Foreground(tcell.ColorYellow).Underline(true))
	drawString(u.screen, boxX+2, boxY+boxH-1, "Press Ctrl-C or Esc to exit")
	drawStringLeft(u.screen, boxX+boxW-1, boxY+boxH-1, fmt.Sprintf("Requests: %d", u.requestCount))
	u.screen.Show()
}

func (u *LeapUI) pollEvents() {
	for {
		ev := u.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEscape, tcell.KeyCtrlC:
				u.cancelFunc()
				return
			}
		}
	}
}
