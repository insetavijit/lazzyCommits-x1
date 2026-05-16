package ipc

type StatusRequest struct{}

type RepoStatus struct {
	Path   string `json:"path"`
	State  string `json:"state"`
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
