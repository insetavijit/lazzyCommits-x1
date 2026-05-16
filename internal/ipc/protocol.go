package ipc

type StatusRequest struct{}

type RepoStatus struct {
	Path   string `json:"path"`
	State  string `json:"state"`
}

type StatusResponse struct {
	Repos []RepoStatus `json:"repos"`
}
