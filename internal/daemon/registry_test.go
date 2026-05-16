package daemon

import (
	"testing"

	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	repo1 := config.RepoConfig{Path: "/repo1", Enabled: true}
	repo2 := config.RepoConfig{Path: "/repo2", Enabled: true}

	cancel1 := func() {}
	cancel2 := func() {}

	r.Add(repo1, cancel1)
	r.Add(repo2, cancel2)

	all := r.All()
	assert.Len(t, all, 2)

	r.Remove("/repo1")
	all = r.All()
	assert.Len(t, all, 1)
	assert.Equal(t, "/repo2", all[0].Config.Path)
}
