package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dnsge/leap/common"
	"github.com/gorilla/websocket"
	"math/rand"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var allChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func generateToken(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = allChars[rand.Intn(len(allChars))]
	}
	return string(b)
}

type Tunnel struct {
	subdomain string
	token     string

	mu           sync.Mutex
	responseChan chan *common.ResponseDataMessage
	errorChan    chan *common.ResponseErrorMessage

	ws *websocket.Conn
}

func newTunnel(subdomain string) *Tunnel {
	return &Tunnel{
		subdomain:    subdomain,
		token:        generateToken(64),
		responseChan: make(chan *common.ResponseDataMessage),
		errorChan:    make(chan *common.ResponseErrorMessage),
		ws:           nil,
	}
}

func (t *Tunnel) sendRawRequest(b []byte) error {
	if t.ws == nil {
		panic("trying to send data on nil connection")
	}

	return t.ws.WriteJSON(common.RequestMessage{
		Data: base64.StdEncoding.EncodeToString(b),
	})
}

func (t *Tunnel) handleRawMessage(data []byte) error {
	var message common.WSMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}

	msgType := message.MessageType()

	switch msgType {
	case common.ResponseData:
		var r common.ResponseDataMessage
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.responseChan <- &r
	case common.ResponseError:
		var r common.ResponseErrorMessage
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.errorChan <- &r
	default:
		return fmt.Errorf("handleRawMessage: unexpected message type %v", msgType)
	}

	return nil
}

func (t *Tunnel) setTunnelConnection(conn *websocket.Conn) {
	t.mu.Lock()
	t.ws = conn
	t.mu.Unlock()
}
