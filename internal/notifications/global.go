package notifications

import "sync"

var (
	globalMu  sync.RWMutex
	globalHub Hub = NewMemoryHub()
)

// SetHub replaces the shared hub instance and returns the previous hub.
func SetHub(h Hub) Hub {
	globalMu.Lock()
	defer globalMu.Unlock()
	prev := globalHub
	if h == nil {
		globalHub = NewMemoryHub()
	} else {
		globalHub = h
	}
	return prev
}

// GetHub returns the shared hub instance.
func GetHub() Hub {
	globalMu.RLock()
	h := globalHub
	globalMu.RUnlock()
	return h
}
