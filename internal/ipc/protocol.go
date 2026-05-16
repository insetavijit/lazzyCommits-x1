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

type ScheduleRequest struct {
	RepoPath string `json:"repoPath"`
	Delay    string `json:"delay"`
	Message  string `json:"message"`
}

type ScheduleResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
