package model

import "sync"

// TableListener is notified when table data changes.
type TableListener interface {
	OnDataChanged()
}

// TableModel holds rows and notifies listeners on updates.
type TableModel struct {
	mu        sync.RWMutex
	resources []Resource
	listeners []TableListener
}

func NewTableModel() *TableModel {
	return &TableModel{}
}

func (t *TableModel) AddListener(l TableListener) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.listeners = append(t.listeners, l)
}

func (t *TableModel) SetResources(resources []Resource) {
	t.mu.Lock()
	t.resources = resources
	listeners := make([]TableListener, len(t.listeners))
	copy(listeners, t.listeners)
	t.mu.Unlock()

	for _, l := range listeners {
		l.OnDataChanged()
	}
}

func (t *TableModel) Resources() []Resource {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]Resource, len(t.resources))
	copy(result, t.resources)
	return result
}
