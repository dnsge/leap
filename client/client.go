package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dnsge/leap/common"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	// Timeout for connecting to the local port and writing/reading data
	localConnectionTimeout = time.Second * 10

	// Timeout for gracefully disconnecting from the server
	disconnectTimeout = time.Second * 5
)

type State uint8

const (
	GettingToken State = iota
	Connecting
	Connected
	Disconnecting
	Disconnected
)

func (s State) String() string {
	switch s {
	case GettingToken:
		return "Getting Token"
	case Connecting:
		return "Connecting"
	case Connected:
		return "Connected"
	case Disconnecting:
		return "Disconnecting"
	case Disconnected:
		return "Disconnected"
	}

	return "Unknown"
}

var httpClient = &http.Client{
	Timeout: time.Second * 5,
}

type LeapClient struct {
	Config *Config

	ws            *websocket.Conn
	closeReadChan chan bool
	quitChan      chan bool

	state State
	subdomain string

	OnError       func(error)
	OnConnect     func()
	OnDisconnect  func()
	OnStateChange func(State)
	OnRequest     func(r *http.Request)
}

func New(config *Config) *LeapClient {
	return &LeapClient{
		Config:        config,
		closeReadChan: make(chan bool),
		quitChan:      make(chan bool),

		state: Disconnected,
		subdomain: "?",
	}
}

func (c *LeapClient) Subdomain() string {
	return c.subdomain
}

func (c *LeapClient) SetState(state State) {
	if c.state != state {
		c.state = state
		if c.OnStateChange != nil {
			c.OnStateChange(state)
		}
	}
}

func (c *LeapClient) Run(ctx context.Context) error {
	c.SetState(GettingToken)
	token, err := c.requestConnectToken()
	if err != nil {
		return fmt.Errorf("connect token: %w", err)
	}

	c.subdomain = token.Subdomain
	c.SetState(Connecting)
	if err := c.dialWebsocket(ctx, token.Token); err != nil {
		return err
	}

	c.SetState(Connected)
	dataChan := c.startReader()
	for {
		select {
		case data := <-dataChan:
			if data == nil {
				continue
			}

			if err := c.handleRequest(data); err != nil {
				if c.OnError != nil {
					c.OnError(fmt.Errorf("request error: %w", err))
				}
			}
		case <-ctx.Done():
			if err := c.disconnectWebsocket(websocket.CloseNormalClosure); err != nil {
				if c.OnError != nil {
					c.OnError(fmt.Errorf("close error: %w", err))
				}
			}
			return err
		case <-c.quitChan:
			return nil
		}
	}
}

func (c *LeapClient) dialWebsocket(ctx context.Context, token string) error {
	// Build URL with access token
	wsURL := c.Config.getWsURL("/api/connect") + "?token=" + token
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial ws: %w", err)
	}

	c.ws = ws
	return nil
}

func (c *LeapClient) startReader() <-chan *common.RequestMessage {
	dataChan := make(chan *common.RequestMessage)
	go func() {
		for {
			var data common.RequestMessage
			if err := c.ws.ReadJSON(&data); err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					c.closeReadChan <- true
				} else {
					if !websocket.IsCloseError(err, websocket.CloseGoingAway) {
						log.Printf("Unexpectedly disconnected from leap server: %v\n", err)
					}
					// just terminate the connection
					c.SetState(Disconnected)
					c.quitChan <- true
					_ = c.ws.Close()
				}
				return
			} else {
				dataChan <- &data
			}
		}
	}()
	return dataChan
}

func (c *LeapClient) disconnectWebsocket(code int) error {
	c.SetState(Disconnecting)

	// https://github.com/gorilla/websocket/issues/448
	oneSecDeadline := time.Now().Add(disconnectTimeout)
	closeMsg := websocket.FormatCloseMessage(code, "closing")
	err := c.ws.WriteControl(websocket.CloseMessage, closeMsg, oneSecDeadline)
	if err != nil && err != websocket.ErrCloseSent {
		return c.ws.Close()
	}

	select {
	case <-c.closeReadChan:
	case <-time.After(disconnectTimeout):
		break
	}

	c.SetState(Disconnected)
	return c.ws.Close()
}

func (c *LeapClient) requestConnectToken() (*common.TokenResponse, error) {
	payload := common.SubdomainRequest{
		Subdomain: c.Config.Subdomain,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	tokenURL := c.Config.getHttpURL("/api/tunnel")
	resp, err := httpClient.Post(tokenURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("token fetch: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusConflict {
			return nil, ErrSubdomainOccupied
		} else {
			body, _ := ioutil.ReadAll(resp.Body)
			fmt.Printf("%d %s\n", resp.StatusCode, string(body))
			return nil, ErrConnectTokenFailed
		}
	}

	var token common.TokenResponse
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if err := json.Unmarshal(b, &token); err != nil {
		return nil, fmt.Errorf("unmarshal body: %w", err)
	}

	return &token, nil
}

func (c *LeapClient) handleRequest(data *common.RequestMessage) error {
	dialLocal := func() (net.Conn, error) {
		d := net.Dialer{
			Timeout:   time.Second * 5,
			KeepAlive: -1,
		}
		return d.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", c.Config.LocalPort))
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(data.Data)
	if err != nil {
		_ = c.ws.WriteJSON(common.NewResponseErrorMessage(common.InternalError))
		return fmt.Errorf("decode data: %w", err)
	}

	if c.OnRequest != nil {
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(decodedBytes)))
		if err == nil {
			go c.OnRequest(req)
		} else {
			fmt.Println(err)
		}
	}

	localConn, err := dialLocal()
	if err != nil {
		_ = c.ws.WriteJSON(common.NewResponseErrorMessage(common.Unavailable))
		return fmt.Errorf("dial local: %w", err)
	}
	defer localConn.Close()
	_ = localConn.SetDeadline(time.Now().Add(localConnectionTimeout))

	_, err = localConn.Write(decodedBytes)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			_ = c.ws.WriteJSON(common.NewResponseErrorMessage(common.Timeout))
			return ErrTimeout
		} else {
			_ = c.ws.WriteJSON(common.NewResponseErrorMessage(common.InternalError))
			return fmt.Errorf("write local: %w", err)
		}
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, localConn)

	encodedBytes := base64.StdEncoding.EncodeToString(buf.Bytes())
	_ = c.ws.WriteJSON(common.NewResponseDataMessage(encodedBytes))
	return nil
}
