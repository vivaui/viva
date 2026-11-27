package platform

import (
	"crypto/rand"
	"sync"
)

type NotificationEvents struct {
	Activated func()
	Failed    func(error)
}

// NotificationRegistry owns active-process callbacks keyed by opaque identities.
// Cancellation removes the callback before a late native response can use it.
type NotificationRegistry struct {
	mu     sync.Mutex
	values map[string]NotificationEvents
}

func (callbacks *NotificationRegistry) Add(events NotificationEvents) string {
	id := rand.Text()
	callbacks.mu.Lock()
	defer callbacks.mu.Unlock()
	if callbacks.values == nil {
		callbacks.values = make(map[string]NotificationEvents)
	}
	callbacks.values[id] = events
	return id
}
func (callbacks *NotificationRegistry) Has(id string) bool {
	callbacks.mu.Lock()
	defer callbacks.mu.Unlock()
	_, exists := callbacks.values[id]
	return exists
}
func (callbacks *NotificationRegistry) Take(id string) NotificationEvents {
	callbacks.mu.Lock()
	defer callbacks.mu.Unlock()
	events := callbacks.values[id]
	delete(callbacks.values, id)
	return events
}
func (callbacks *NotificationRegistry) Remove(id string) { callbacks.Take(id) }
