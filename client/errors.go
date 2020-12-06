package client

import "errors"

var (
	ErrTimeout = errors.New("connection timed out")
	ErrSubdomainOccupied = errors.New("subdomain occupied")
	ErrConnectTokenFailed = errors.New("failed to obtain connect token")
)
