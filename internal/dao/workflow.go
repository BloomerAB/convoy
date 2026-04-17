package dao

import (
	"context"

	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/model"
)

// WorkflowDAO wraps the GitHub poller behind the Watcher interface.
type WorkflowDAO struct {
	poller *client.GitHubPoller
}

func NewWorkflowDAO(poller *client.GitHubPoller) *WorkflowDAO {
	return &WorkflowDAO{poller: poller}
}

func (d *WorkflowDAO) Start(ctx context.Context) error {
	return d.poller.Start(ctx)
}

func (d *WorkflowDAO) Resources() []model.Resource {
	return d.poller.Resources()
}
