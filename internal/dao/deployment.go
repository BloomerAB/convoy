package dao

import (
	"context"

	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/model"
)

type DeploymentDAO struct {
	watcher *client.DeploymentWatcher
}

func NewDeploymentDAO(clusterClient *client.ClusterClient) *DeploymentDAO {
	return &DeploymentDAO{
		watcher: client.NewDeploymentWatcher(clusterClient),
	}
}

func (d *DeploymentDAO) Start(ctx context.Context) error {
	return d.watcher.Start(ctx)
}

func (d *DeploymentDAO) Resources() []model.Resource {
	return d.watcher.Resources()
}
