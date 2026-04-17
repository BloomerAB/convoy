package dao

import (
	"context"

	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/model"
)

type HelmRepositoryDAO struct {
	watcher *client.FluxWatcher
}

func NewHelmRepositoryDAO(clusterClient *client.ClusterClient, onChange func()) *HelmRepositoryDAO {
	return &HelmRepositoryDAO{
		watcher: client.NewFluxWatcher(clusterClient, client.HelmRepositoryGVR, model.KindHelmRepository, onChange),
	}
}

func (d *HelmRepositoryDAO) Start(ctx context.Context) error {
	return d.watcher.Start(ctx)
}

func (d *HelmRepositoryDAO) Resources() []model.Resource {
	return d.watcher.Resources()
}
