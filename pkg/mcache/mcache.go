// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package mcache

import (
	"sync"
	"time"
)

const (
	NoExpiration time.Duration = -1
)

func New[K comparable, V any](expiration time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		expiry: expiration,
		items:  make(map[K]item[V]),
		now:    time.Now,
	}
}

type Cache[K comparable, V any] struct {
	expiry time.Duration
	mu     sync.RWMutex
	items  map[K]item[V]
	now    func() time.Time
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.Unlock()

	if item, ok := c.items[key]; ok {
		now := c.now().UnixNano()
		if !item.expired(now) {
			return item.value, true
		}
	}

	var empty V
	return empty, false
}

func (c *Cache[K, V]) Set(key K, value V, d ...time.Duration) {
	expiry := c.expiry
	if len(d) > 0 {
		expiry = d[0]
	}

	var expires int64
	if expiry > 0 {
		expires = c.now().Add(expiry).UnixNano()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = item[V]{
		value, expires,
	}
}

func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.items[key]; ok {
		delete(c.items, key)
		return true
	}
	return false
}

func (c *Cache[K, V]) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

func (c *Cache[K, V]) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var count int
	now := c.now().UnixNano()
	for k, i := range c.items {
		if i.expired(now) {
			delete(c.items, k)
			count++
		}
	}
	return count
}

func (c *Cache[K, V]) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[K]item[V])
}

func StartCleanupLoop(c interface{ Cleanup() int }, interval time.Duration) (stop func()) {
	var wg sync.WaitGroup
	sc := make(chan struct{})
	go loop(c.Cleanup, interval, sc, &wg)
	return func() {
		close(sc)
		wg.Wait()
	}
}

func loop(cleanup func() int, d time.Duration, stop chan struct{}, wg *sync.WaitGroup) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-ticker.C:
			cleanup()
		case <-stop:
			return
		}
	}
}

type item[V any] struct {
	value   V
	expires int64
}

func (i *item[V]) expired(now int64) bool {
	if i.expires == 0 {
		return false
	}
	return now > i.expires
}
