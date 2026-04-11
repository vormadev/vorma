package mailbox

import "sync"

type Mailbox[K comparable, V any] struct {
	mu     sync.Mutex
	mail   map[K]V
	notify chan struct{}
}

func NewMailbox[K comparable, V any]() *Mailbox[K, V] {
	return &Mailbox[K, V]{
		mail:   make(map[K]V),
		notify: make(chan struct{}, 1),
	}
}

func (m *Mailbox[K, V]) Deliver(k K, v V) {
	m.mu.Lock()
	m.mail[k] = v
	m.mu.Unlock()

	select {
	case m.notify <- struct{}{}:
	default:
	}
}

func (m *Mailbox[K, V]) Claim() map[K]V {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.mail) == 0 {
		return nil
	}
	snapshot := m.mail
	m.mail = make(map[K]V)
	return snapshot
}

func (m *Mailbox[K, V]) C() <-chan struct{} { return m.notify }
