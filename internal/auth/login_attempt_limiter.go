package auth

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidLoginAttemptCapacity = errors.New(
		"login attempt capacity must be positive",
	)
	ErrInvalidLoginAttemptRefillInterval = errors.New(
		"login attempt refill interval must be positive",
	)
	ErrInvalidLoginAttemptMaxEntries = errors.New(
		"login attempt max entries must be positive",
	)
)

type LoginAttemptLimiter interface {
	Allow(identifier string) bool
	Succeeded(identifier string)
}

type LoginAttemptLimiterConfig struct {
	Capacity       int
	RefillInterval time.Duration
	MaxEntries     int
}

type InMemoryLoginAttemptLimiter struct {
	mu             sync.Mutex
	capacity       int
	refillInterval time.Duration
	maxEntries     int
	entries        map[string]*list.Element
	order          *list.List
	now            func() time.Time
}

type loginAttemptLimiterEntry struct {
	identifier string
	tokens     int
	lastRefill time.Time
}

func NewInMemoryLoginAttemptLimiter(
	config LoginAttemptLimiterConfig,
) (*InMemoryLoginAttemptLimiter, error) {
	if config.Capacity <= 0 {
		return nil, ErrInvalidLoginAttemptCapacity
	}

	if config.RefillInterval <= 0 {
		return nil, ErrInvalidLoginAttemptRefillInterval
	}

	if config.MaxEntries <= 0 {
		return nil, ErrInvalidLoginAttemptMaxEntries
	}

	return &InMemoryLoginAttemptLimiter{
		capacity:       config.Capacity,
		refillInterval: config.RefillInterval,
		maxEntries:     config.MaxEntries,
		entries:        make(map[string]*list.Element),
		order:          list.New(),
		now:            time.Now,
	}, nil
}

func (l *InMemoryLoginAttemptLimiter) Allow(
	identifier string,
) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	element, ok := l.entries[identifier]
	if !ok {
		if len(l.entries) >= l.maxEntries {
			l.evictOldest()
		}

		entry := &loginAttemptLimiterEntry{
			identifier: identifier,
			tokens:     l.capacity,
			lastRefill: now,
		}

		element = l.order.PushBack(entry)
		l.entries[identifier] = element
	}

	l.order.MoveToBack(element)

	entry := element.Value.(*loginAttemptLimiterEntry)

	l.refill(entry, now)

	if entry.tokens == 0 {
		return false
	}

	entry.tokens--

	return true
}

func (l *InMemoryLoginAttemptLimiter) Succeeded(
	identifier string,
) {
	l.mu.Lock()
	defer l.mu.Unlock()

	element, ok := l.entries[identifier]
	if !ok {
		return
	}

	delete(l.entries, identifier)
	l.order.Remove(element)
}

func (l *InMemoryLoginAttemptLimiter) refill(
	entry *loginAttemptLimiterEntry,
	now time.Time,
) {
	if entry.tokens == l.capacity ||
		!now.After(entry.lastRefill) {
		return
	}

	elapsed := now.Sub(entry.lastRefill)
	refillCount := int(elapsed / l.refillInterval)

	if refillCount == 0 {
		return
	}

	missing := l.capacity - entry.tokens

	if refillCount >= missing {
		entry.tokens = l.capacity
		entry.lastRefill = now
		return
	}

	entry.tokens += refillCount
	entry.lastRefill = entry.lastRefill.Add(
		time.Duration(refillCount) * l.refillInterval,
	)
}

func (l *InMemoryLoginAttemptLimiter) evictOldest() {
	element := l.order.Front()
	if element == nil {
		return
	}

	entry := element.Value.(*loginAttemptLimiterEntry)

	delete(l.entries, entry.identifier)
	l.order.Remove(element)
}

var _ LoginAttemptLimiter = (*InMemoryLoginAttemptLimiter)(nil)
