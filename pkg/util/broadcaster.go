package util

import (
	"sync"
)

// KeyedBroadcaster notifies every subscriber for a key about a change.
// Multiple broadcasts are coalesced until a subscriber receives the signal.
type KeyedBroadcaster[K comparable] interface {
	Broadcast(key K)
	Subscribe(key K) (<-chan struct{}, func())
}

type keyedBroadcaster[K comparable] struct {
	mutex       sync.Mutex
	subscribers map[K]map[chan struct{}]struct{}
}

func NewKeyedBroadcaster[K comparable]() KeyedBroadcaster[K] {
	return &keyedBroadcaster[K]{
		subscribers: make(map[K]map[chan struct{}]struct{}),
	}
}

var _ KeyedBroadcaster[string] = (*keyedBroadcaster[string])(nil)

func (b *keyedBroadcaster[K]) Broadcast(key K) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for subscriber := range b.subscribers[key] {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

// Subscribe returns a channel that receives broadcasts for key and an
// idempotent function that removes the subscription.
func (b *keyedBroadcaster[K]) Subscribe(key K) (<-chan struct{}, func()) {
	subscriber := make(chan struct{}, 1)

	b.mutex.Lock()
	if b.subscribers[key] == nil {
		b.subscribers[key] = make(map[chan struct{}]struct{})
	}
	b.subscribers[key][subscriber] = struct{}{}
	b.mutex.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mutex.Lock()
			defer b.mutex.Unlock()

			delete(b.subscribers[key], subscriber)
			if len(b.subscribers[key]) == 0 {
				delete(b.subscribers, key)
			}
		})
	}

	return subscriber, unsubscribe
}
