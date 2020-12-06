package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dnsge/leap/common"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LeapServer struct {
	config  *Config
	mu      sync.Mutex
	tunnels map[string]*Tunnel
	ctx     context.Context
}

func New(config *Config) *LeapServer {
	return &LeapServer{
		config:  config,
		tunnels: make(map[string]*Tunnel),
		ctx:     context.Background(),
	}
}

func (s *LeapServer) makeServer() *http.Server {
	if s.config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(s.interceptRequest)
	r.GET("/api/status", s.getStatus)
	r.POST("/api/tunnel", s.newTunnelRequest)
	r.GET("/api/connect", s.connectTunnel)

	server := &http.Server{Addr: s.config.Bind}
	go func() {
		log.Println("Starting leap server")
		if err := http.ListenAndServe(s.config.Bind, r); err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	return server
}

func (s *LeapServer) Run(ctx context.Context) error {
	s.ctx = ctx
	server := s.makeServer()

	<-ctx.Done() // wait for interrupt
	s.mu.Lock()
	defer s.mu.Unlock()

	closeMessage := websocket.FormatCloseMessage(websocket.CloseGoingAway, "closing")
	for _, tun := range s.tunnels {
		if tun.ws != nil {
			expire := time.Now().Add(time.Millisecond * 500)
			_ = tun.ws.WriteControl(websocket.CloseMessage, closeMessage, expire)
		}
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return server.Shutdown(timeout)
}

func (s *LeapServer) interceptRequest(c *gin.Context) {
	if c.Request.Host == s.config.Domain { // request to base domain (API request)
		c.Next()
	} else {
		c.Abort() // prevent other handlers from being called
		if strings.HasSuffix(c.Request.Host, s.config.Domain) {
			subdomain := strings.Split(c.Request.Host, ".")[0]
			if tun, ok := s.tunnels[subdomain]; ok {
				err := passExternalRequest(c, tun)
				if err != nil {
					log.Println("external error:", err)
				}
				return
			}
		}
		c.Status(http.StatusNotFound)
	}
}

type statusResponse struct {
	Subdomains int `json:"subdomains"`
}

func (s *LeapServer) getStatus(c *gin.Context) {
	resp := statusResponse{
		Subdomains: len(s.tunnels),
	}
	c.JSON(http.StatusOK, resp)
}

var wsUpgrade = websocket.Upgrader{
	HandshakeTimeout: time.Second * 10,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

func readSubdomainRequest(r *http.Request) (*common.SubdomainRequest, error) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	var payload common.SubdomainRequest
	if err = json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}

	// enforce lowercase subdomains
	payload.Subdomain = strings.ToLower(payload.Subdomain)

	return &payload, nil
}

func (s *LeapServer) isSubdomainAvailable(sub string) bool {
	_, ok := s.tunnels[strings.ToLower(sub)]
	return !ok
}

func (s *LeapServer) createNewTunnel(sub string) *Tunnel {
	newTun := newTunnel(sub)
	s.tunnels[sub] = newTun
	return newTun
}

func (s *LeapServer) newTunnelRequest(c *gin.Context) {
	sr, err := readSubdomainRequest(c.Request)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var subdomain string
	if sr.Subdomain == "" { // wants random
		randomSub := randomSubdomain()
		for !s.isSubdomainAvailable(randomSub) {
			randomSub = randomSubdomain()
		}
		subdomain = randomSub
	} else { // wants specific
		if !s.isSubdomainAvailable(sr.Subdomain) {
			c.String(http.StatusConflict, "A tunnel on the subdomain %q already exists", sr.Subdomain)
			return
		}
		subdomain = sr.Subdomain
	}

	newTun := s.createNewTunnel(subdomain)
	token := common.TokenResponse{
		Token:     newTun.token,
		Subdomain: subdomain,
	}

	c.JSON(http.StatusOK, token)
}

func (s *LeapServer) connectTunnel(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "Missing token query argument")
		return
	}

	found := false
	for _, tun := range s.tunnels {
		if tun.token == token {
			conn, err := wsUpgrade.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				fmt.Println(err)
				_ = c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			found = true
			tun.setTunnelConnection(conn)
			go s.handleTunnelConnection(tun)
		}
	}

	if !found {
		c.String(http.StatusBadRequest, "Invalid token")
	}
}

func (s *LeapServer) handleTunnelConnection(tun *Tunnel) {
	defer func() {
		if s.config.Debug {
			log.Printf("Client %q disconnected\n", tun.subdomain)
		}
		_ = tun.ws.Close()
		delete(s.tunnels, tun.subdomain)
	}()

	if s.config.Debug {
		log.Printf("Client %q connected\n", tun.subdomain)
	}

	for {
		_, data, err := tun.ws.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Println("read:", err)
			} else {
				_ = tun.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing"), time.Now().Add(time.Second))
			}
			break
		}
		if err := tun.handleRawMessage(data); err != nil {
			log.Println("handle:", err)
		}
	}
}

func randomSubdomain() string {
	var hexDigits = []rune("0123456789abcdef")
	b := make([]rune, 6)
	for i := range b {
		b[i] = hexDigits[rand.Intn(len(hexDigits))]
	}
	return string(b)
}
