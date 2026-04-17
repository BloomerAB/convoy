package dao

import (
	"context"

	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/model"
)

type HelmReleaseDAO struct {
	watcher *client.FluxWatcher
}

func NewHelmReleaseDAO(clusterClient *client.ClusterClient, onChange func()) *HelmReleaseDAO {
	return &HelmReleaseDAO{
		watcher: client.NewFluxWatcher(clusterClient, client.HelmReleaseGVR, model.KindHelmRelease, onChange),
	}
}

func (d *HelmReleaseDAO) Start(ctx context.Context) error {
	return d.watcher.Start(ctx)
}

func (d *HelmReleaseDAO) Resources() []model.Resource {
	return d.watcher.Resources()
}
