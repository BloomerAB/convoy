package dao

import (
	"context"

	"github.com/bloomerab/convoy/internal/model"
)

// Watcher provides a uniform interface for data sources.
type Watcher interface {
	Start(ctx context.Context) error
	Resources() []model.Resource
}
