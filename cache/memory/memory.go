// Package memory implements a bounded in-memory key-value cache.
package memory

import (
	"container/list"
	"context"
	"strings"
	"sync"
	"time"

	"sifeucr/cache"
)

const (
	defaultTTL        = 20 * time.Second
	defaultMaxEntries = 1024
	defaultMaxBytes   = 8 << 20
)

// OptFunc configures the store through the With* options provided by this package.
type OptFunc func(*memoryCache)

type Store struct {
	mu sync.Mutex
	memoryCache
	items  map[string]*list.Element
	order  list.List
	bytes  int
	closed bool
	stop   chan struct{}
	done   chan struct{}
}

var _ cache.Store = (*Store)(nil)

// New starts background expiration. The caller must Close the store when done.
// Capacity is bounded by both entry count and total key/value bytes; Go object
// overhead is additional but bounded by the entry limit. Eviction removes the
// oldest written entry. Reads do not extend TTL or change eviction order.
func New(opts ...OptFunc) *Store {
	c := &Store{
		memoryCache: memoryCache{ttl: defaultTTL, maxEntries: defaultMaxEntries, maxBytes: defaultMaxBytes},
		items:       make(map[string]*list.Element),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(&c.memoryCache)
	}
	go c.clean(min(c.ttl, time.Second))
	return c
}

func WithTTL(ttl time.Duration) OptFunc {
	return func(c *memoryCache) {
		if ttl > 0 {
			c.ttl = ttl
		}
	}
}

func WithMaxEntries(limit int) OptFunc {
	return func(c *memoryCache) {
		if limit > 0 {
			c.maxEntries = limit
		}
	}
}

// WithMaxBytes limits retained key/value bytes, not total process memory.
func WithMaxBytes(limit int) OptFunc {
	return func(c *memoryCache) {
		if limit > 0 {
			c.maxBytes = limit
		}
	}
}

func (c *Store) Set(ctx context.Context, key string, value []byte) error {
	if err := validateRequest(ctx, key); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return cache.ErrClosed
	}
	if len(key) > c.maxBytes || len(value) > c.maxBytes-len(key) {
		return cache.ErrTooLarge
	}
	now := time.Now()
	c.deleteExpired(now)
	if existing := c.items[key]; existing != nil {
		c.remove(existing)
	}
	size := len(key) + len(value)
	for len(c.items) >= c.maxEntries || c.bytes > c.maxBytes-size {
		c.remove(c.order.Front())
	}
	// Detach keys as well as values from potentially much larger caller buffers.
	item := &entry{key: strings.Clone(key), value: append([]byte(nil), value...), expiresAt: now.Add(c.ttl)}
	c.items[item.key] = c.order.PushBack(item)
	c.bytes += size
	return nil
}

func (c *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := validateRequest(ctx, key); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, cache.ErrClosed
	}
	c.deleteExpired(time.Now())
	element := c.items[key]
	if element == nil {
		return nil, cache.ErrNotFound
	}
	return append([]byte(nil), element.Value.(*entry).value...), nil
}

func (c *Store) Delete(ctx context.Context, key string) error {
	if err := validateRequest(ctx, key); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return cache.ErrClosed
	}
	if element := c.items[key]; element != nil {
		c.remove(element)
	}
	return nil
}

// Stats reports retained entries and key/value bytes without triggering cleanup.
func (c *Store) Stats() (entries, bytes int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items), c.bytes
}

// Close stops cleanup, releases entries, and rejects further operations.
// It is safe to call repeatedly or concurrently.
func (c *Store) Close() error {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		clear(c.items)
		c.order.Init()
		c.bytes = 0
		close(c.stop)
	}
	c.mu.Unlock()
	<-c.done
	return nil
}
