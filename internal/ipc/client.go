package ipc

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
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
	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	envelope := struct {
		Type    string `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "status",
	}

	if err := json.NewEncoder(conn).Encode(envelope); err != nil {
		return nil, err
	}

	var resp StatusResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) ScheduleCommit(path, delay, msg string) (*ScheduleResponse, error) {
	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	envelope := struct {
		Type    string `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "schedule",
		Payload: ScheduleRequest{
			RepoPath: path,
			Delay:    delay,
			Message:  msg,
		},
	}

	if err := json.NewEncoder(conn).Encode(envelope); err != nil {
		return nil, err
	}

	var resp ScheduleResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

