package resolver

import (
	"sync"
	"time"
)

const (
	defaultCacheSize = 1000
)

type cacheEntry struct {
	result   *Result
	expiry   time.Time
	lastUsed time.Time
}

type cacheNode struct {
	key   string
	entry *cacheEntry
	prev  *cacheNode
	next  *cacheNode
}

type dnsCache struct {
	mutex    sync.RWMutex
	capacity int
	cache    map[string]*cacheNode
	head     *cacheNode
	tail     *cacheNode
}

func newDNSCache(capacity int) *dnsCache {
	if capacity <= 0 {
		capacity = defaultCacheSize
	}

	head := &cacheNode{}
	tail := &cacheNode{}
	head.next = tail
	tail.prev = head

	return &dnsCache{
		capacity: capacity,
		cache:    make(map[string]*cacheNode),
		head:     head,
		tail:     tail,
	}
}

func (c *dnsCache) get(key string) (*Result, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	node, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(node.entry.expiry) {
		c.removeNode(node)
		delete(c.cache, key)
		return nil, false
	}

	node.entry.lastUsed = time.Now()
	c.moveToHead(node)

	return node.entry.result, true
}

func (c *dnsCache) put(key string, result *Result, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	entry := &cacheEntry{
		result:   result,
		expiry:   now.Add(ttl),
		lastUsed: now,
	}

	if node, exists := c.cache[key]; exists {
		node.entry = entry
		c.moveToHead(node)
		return
	}

	newNode := &cacheNode{
		key:   key,
		entry: entry,
	}

	c.cache[key] = newNode
	c.addToHead(newNode)

	if len(c.cache) > c.capacity {
		tail := c.removeTail()
		delete(c.cache, tail.key)
	}
}

func (c *dnsCache) addToHead(node *cacheNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

func (c *dnsCache) removeNode(node *cacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (c *dnsCache) moveToHead(node *cacheNode) {
	c.removeNode(node)
	c.addToHead(node)
}

func (c *dnsCache) removeTail() *cacheNode {
	lastNode := c.tail.prev
	c.removeNode(lastNode)
	return lastNode
}
