package resolver

import (
	"sync"
	"time"
)

const (
	defaultCacheSize = 1000
)

// cacheEntry represents a single cached DNS result with metadata.
type cacheEntry struct {
	result   *Result  // The cached DNS query result
	expiry   time.Time // When this cache entry expires
	lastUsed time.Time // When this entry was last accessed (for LRU)
	negative bool     // true for NXDOMAIN/negative cache entries
}

// cacheNode is a node in the doubly-linked list used for LRU cache implementation.
type cacheNode struct {
	key   string      // Cache key for this entry
	entry *cacheEntry // The cached data
	prev  *cacheNode  // Previous node in the linked list
	next  *cacheNode  // Next node in the linked list
}

// dnsCache implements an LRU cache for DNS query results.
// It uses a combination of a hash map for O(1) lookups and a doubly-linked
// list for O(1) LRU operations.
type dnsCache struct {
	mutex    sync.RWMutex           // Protects all cache operations
	capacity int                    // Maximum number of entries to store
	cache    map[string]*cacheNode  // Hash map for fast lookups
	head     *cacheNode             // Head of doubly-linked list (most recent)
	tail     *cacheNode             // Tail of doubly-linked list (least recent)
}

// newDNSCache creates a new DNS cache with the specified capacity.
// If capacity is <= 0, it defaults to defaultCacheSize.
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

// get retrieves a cached DNS result by key.
// It checks for expiration and updates the LRU order on cache hits.
// Returns (result, found) where found indicates if the key was in cache and not expired.
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

// put stores a DNS result in the cache with the specified TTL.
// This is a convenience wrapper around putWithNegative for positive cache entries.
func (c *dnsCache) put(key string, result *Result, ttl time.Duration) {
	c.putWithNegative(key, result, ttl, false)
}

// putNegative stores a negative DNS result (like NXDOMAIN) in the cache.
// Negative entries are handled specially for cache management purposes.
func (c *dnsCache) putNegative(key string, result *Result, ttl time.Duration) {
	c.putWithNegative(key, result, ttl, true)
}

// putWithNegative is the internal implementation for storing cache entries.
// It handles both positive and negative cache entries, manages LRU ordering,
// and evicts old entries when the cache reaches capacity.
func (c *dnsCache) putWithNegative(key string, result *Result, ttl time.Duration, negative bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	entry := &cacheEntry{
		result:   result,
		expiry:   now.Add(ttl),
		lastUsed: now,
		negative: negative,
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

// addToHead adds a node to the head of the doubly-linked list (most recent position).
func (c *dnsCache) addToHead(node *cacheNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNode removes a node from the doubly-linked list.
func (c *dnsCache) removeNode(node *cacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// moveToHead moves an existing node to the head of the list (marks as most recently used).
func (c *dnsCache) moveToHead(node *cacheNode) {
	c.removeNode(node)
	c.addToHead(node)
}

// removeTail removes and returns the node at the tail of the list (least recently used).
func (c *dnsCache) removeTail() *cacheNode {
	lastNode := c.tail.prev
	c.removeNode(lastNode)
	return lastNode
}
