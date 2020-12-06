package server

import (
	"fmt"
	"github.com/dnsge/leap/common"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httputil"
)

func passExternalRequest(c *gin.Context, tun *Tunnel) error {
	// lock the tunnel to preserve request/response ordering
	tun.mu.Lock()
	defer tun.mu.Unlock()

	c.Request.Header.Set("Connection", "close")
	rawRequest, err := httputil.DumpRequest(c.Request, true)
	if err != nil {
		return fmt.Errorf("dump request: %w", err)
	}

	if err := tun.sendRawRequest(rawRequest); err != nil {
		return fmt.Errorf("sendRawRequest: %w", err)
	}

	select {
	case respMsg := <-tun.responseChan:
		return proxyResponse(c, respMsg)
	case errMsg := <-tun.errorChan:
		handleError(c, errMsg)
		return nil
	}
}

func proxyResponse(c *gin.Context, resp *common.ResponseDataMessage) error {
	decodedBytes, err := resp.DecodeResponse()
	if err != nil {
		c.String(http.StatusInternalServerError, "Bad encoding: %v", err)
		return fmt.Errorf("readRawResponse: %w", err)
	}

	conn, buffer, err := c.Writer.Hijack()
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return fmt.Errorf("hijack: %w", err)
	}

	_, _ = buffer.Write(decodedBytes)
	_ = buffer.Flush()
	_ = conn.Close()
	return nil
}

func handleError(c *gin.Context, e *common.ResponseErrorMessage) {
	switch e.Code {
	case common.Unavailable:
		c.String(http.StatusServiceUnavailable, "Failed to connect to local service")
	case common.InternalError:
		c.String(http.StatusInternalServerError, "An internal error occurred while proxying the request")
	case common.Timeout:
		c.String(http.StatusGatewayTimeout, "The local service took too long to respond")
	}
}
