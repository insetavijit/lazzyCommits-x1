package ipc

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Client struct {
	socket string
}

func NewClient() (*Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	socket := filepath.Join(home, ".lazycommit", "daemon.sock")
	return &Client{socket: socket}, nil
}

func (c *Client) GetStatus() (*StatusResponse, error) {
	conn, err := net.DialTimeout("unix", c.socket, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := StatusRequest{}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}

	var resp StatusResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
