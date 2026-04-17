package dao

import (
	"context"

	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/model"
)

// KustomizationDAO watches Kustomization resources on a cluster.
type KustomizationDAO struct {
	watcher *client.FluxWatcher
}

func NewKustomizationDAO(clusterClient *client.ClusterClient, onChange func()) *KustomizationDAO {
	return &KustomizationDAO{
		watcher: client.NewFluxWatcher(clusterClient, client.KustomizationGVR, model.KindKustomization, onChange),
	}
}

func (d *KustomizationDAO) Start(ctx context.Context) error {
	return d.watcher.Start(ctx)
}

func (d *KustomizationDAO) Resources() []model.Resource {
	return d.watcher.Resources()
}
