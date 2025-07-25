package resolver

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestCacheBasicFunctionality(t *testing.T) {
	resolver := NewWithCacheSize(10)
	domain := "google.com"

	result1, err := resolver.Resolve(domain, dns.TypeA)
	if err != nil {
		t.Fatalf("Failed to resolve %s: %v", domain, err)
	}

	start := time.Now()
	result2, err := resolver.Resolve(domain, dns.TypeA)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to resolve cached %s: %v", domain, err)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Cached lookup took too long: %v", duration)
	}

	if len(result1.Answer) != len(result2.Answer) {
		t.Errorf("Cache returned different number of answers")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	cache := newDNSCache(10)

	result := &Result{
		Answer: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "test.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    1,
				},
				A: net.ParseIP("1.2.3.4"),
			},
		},
	}

	cache.put("test.com.:1", result, 1*time.Second)

	if cached, found := cache.get("test.com.:1"); !found {
		t.Error("Should find cached entry immediately")
	} else if len(cached.Answer) != 1 {
		t.Error("Cached result should have 1 answer")
	}

	time.Sleep(1100 * time.Millisecond)

	if _, found := cache.get("test.com.:1"); found {
		t.Error("Should not find expired cached entry")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	cache := newDNSCache(2)

	result1 := &Result{Answer: []dns.RR{}}
	result2 := &Result{Answer: []dns.RR{}}
	result3 := &Result{Answer: []dns.RR{}}

	cache.put("key1", result1, 1*time.Hour)
	cache.put("key2", result2, 1*time.Hour)
	cache.put("key3", result3, 1*time.Hour)

	if _, found := cache.get("key1"); found {
		t.Error("key1 should have been evicted")
	}

	if _, found := cache.get("key2"); !found {
		t.Error("key2 should still be in cache")
	}

	if _, found := cache.get("key3"); !found {
		t.Error("key3 should still be in cache")
	}
}

func TestCacheLRUOrdering(t *testing.T) {
	cache := newDNSCache(3)

	result1 := &Result{Answer: []dns.RR{}}
	result2 := &Result{Answer: []dns.RR{}}
	result3 := &Result{Answer: []dns.RR{}}
	result4 := &Result{Answer: []dns.RR{}}

	cache.put("key1", result1, 1*time.Hour)
	cache.put("key2", result2, 1*time.Hour)
	cache.put("key3", result3, 1*time.Hour)

	cache.get("key1")

	cache.put("key4", result4, 1*time.Hour)

	if _, found := cache.get("key2"); found {
		t.Error("key2 should have been evicted (was least recently used)")
	}

	if _, found := cache.get("key1"); !found {
		t.Error("key1 should still be in cache (was accessed recently)")
	}
}

func TestCacheKeyGeneration(t *testing.T) {
	resolver := New()

	key1 := resolver.makeCacheKey("example.com.", dns.TypeA)
	key2 := resolver.makeCacheKey("example.com.", dns.TypeAAAA)
	key3 := resolver.makeCacheKey("test.com.", dns.TypeA)

	if key1 == key2 {
		t.Error("Different record types should generate different keys")
	}

	if key1 == key3 {
		t.Error("Different domains should generate different keys")
	}

	expected1 := "example.com.:1"
	if key1 != expected1 {
		t.Errorf("Expected key %s, got %s", expected1, key1)
	}
}

func TestCalculateTTL(t *testing.T) {
	resolver := New()

	result := &Result{
		Answer: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Ttl: 300},
			},
			&dns.A{
				Hdr: dns.RR_Header{Ttl: 600},
			},
		},
		Authority: []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{Ttl: 200},
			},
		},
	}

	ttl := resolver.calculateTTL(result)
	expected := 200 * time.Second

	if ttl != expected {
		t.Errorf("Expected TTL %v, got %v", expected, ttl)
	}

	result.Authority[0].Header().Ttl = 30
	ttl = resolver.calculateTTL(result)
	expected = 60 * time.Second

	if ttl != expected {
		t.Errorf("Expected minimum TTL %v, got %v", expected, ttl)
	}
}
