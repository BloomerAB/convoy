package dao

import (
	"context"

	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/model"
)

type GitRepositoryDAO struct {
	watcher *client.FluxWatcher
}

func NewGitRepositoryDAO(clusterClient *client.ClusterClient, onChange func()) *GitRepositoryDAO {
	return &GitRepositoryDAO{
		watcher: client.NewFluxWatcher(clusterClient, client.GitRepositoryGVR, model.KindGitRepository, onChange),
	}
}

func (d *GitRepositoryDAO) Start(ctx context.Context) error {
	return d.watcher.Start(ctx)
}

func (d *GitRepositoryDAO) Resources() []model.Resource {
	return d.watcher.Resources()
}
