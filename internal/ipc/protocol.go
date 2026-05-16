package ipc

import "time"

type StatusRequest struct{}

type RepoStatus struct {
	Path         string    `json:"path"`
	State        string    `json:"state"`
	ScheduledAt  time.Time `json:"scheduledAt,omitempty"`
	ScheduledMsg string    `json:"scheduledMsg,omitempty"`
}

type StatusResponse struct {
	Repos []RepoStatus `json:"repos"`
}

type TaskRequest struct {
	Type    string   `json:"type"` // "commit", "push"
	Repo    string   `json:"repo"`
	Delay   string   `json:"delay"`
	Args    []string `json:"args,omitempty"`
}

type TaskResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ScheduledListRequest struct{}

type ScheduledListResponse struct {
	Tasks []TaskInfo `json:"tasks"`
}

type TaskInfo struct {
	ID    string    `json:"id"`
	Type  string    `json:"type"`
	Repo  string    `json:"repo"`
	RunAt time.Time `json:"runAt"`
}

type TerminateTaskRequest struct {
	ID string `json:"id"`
}

type TerminateTaskResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
