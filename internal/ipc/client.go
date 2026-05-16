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
		Type    string      `json:"type"`
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

func (c *Client) ScheduleTask(repo, taskType, delay string, args []string) (*TaskResponse, error) {
	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	envelope := struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "task",
		Payload: TaskRequest{
			Type:  taskType,
			Repo:  repo,
			Delay: delay,
			Args:  args,
		},
	}

	if err := json.NewEncoder(conn).Encode(envelope); err != nil {
		return nil, err
	}

	var resp TaskResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) GetScheduledTasks() (*ScheduledListResponse, error) {
	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	envelope := struct {
		Type string `json:"type"`
	}{
		Type: "scheduled_list",
	}

	if err := json.NewEncoder(conn).Encode(envelope); err != nil {
		return nil, err
	}

	var resp ScheduledListResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) TerminateTask(id string) (*TerminateTaskResponse, error) {
	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	envelope := struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "terminate_task",
		Payload: TerminateTaskRequest{
			ID: id,
		},
	}

	if err := json.NewEncoder(conn).Encode(envelope); err != nil {
		return nil, err
	}

	var resp TerminateTaskResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
