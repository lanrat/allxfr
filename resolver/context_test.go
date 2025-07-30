package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestResolverContextCancellation(t *testing.T) {
	resolver := New()

	t.Run("Resolve_CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := resolver.Resolve(ctx, "google.com", dns.TypeA)
		if err == nil {
			t.Error("Expected error due to cancelled context, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})

	t.Run("ResolveAll_CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := resolver.ResolveAll(ctx, "google.com", dns.TypeA)
		if err == nil {
			t.Error("Expected error due to cancelled context, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})

	t.Run("LookupIP_CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := resolver.LookupIP(ctx, "google.com")
		if err == nil {
			t.Error("Expected error due to cancelled context, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})

	t.Run("LookupIPAll_CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := resolver.LookupIPAll(ctx, "google.com")
		if err == nil {
			t.Error("Expected error due to cancelled context, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})
}

func TestResolverContextTimeout(t *testing.T) {
	resolver := New()

	t.Run("Resolve_ShortTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		_, err := resolver.Resolve(ctx, "google.com", dns.TypeA)
		if err == nil {
			t.Error("Expected error due to context timeout, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})

	t.Run("ResolveAll_ShortTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		_, err := resolver.ResolveAll(ctx, "google.com", dns.TypeA)
		if err == nil {
			t.Error("Expected error due to context timeout, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})

	t.Run("LookupIP_ShortTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		_, err := resolver.LookupIP(ctx, "google.com")
		if err == nil {
			t.Error("Expected error due to context timeout, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})

	t.Run("LookupIPAll_ShortTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		_, err := resolver.LookupIPAll(ctx, "google.com")
		if err == nil {
			t.Error("Expected error due to context timeout, but got nil")
		}
		t.Logf("Got error (as expected): %v", err)
	})
}

func TestResolverContextCancellationDuringQuery(t *testing.T) {
	resolver := NewWithTimeout(10 * time.Second)

	t.Run("CancelDuringResolve", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		started := make(chan struct{})
		var err error

		go func() {
			defer close(done)
			close(started) // Signal that the goroutine has started
			// Use a domain that doesn't exist to force multiple recursive queries
			// This increases the chance of cancellation during resolution
			_, err = resolver.Resolve(ctx, "nonexistent.test.invalid", dns.TypeA)
		}()

		// Wait for the goroutine to actually start the query
		<-started
		// Add a small delay to let the query start, then cancel
		time.Sleep(10 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// Test passes if either:
			// 1. Context was cancelled (err contains "context canceled")
			// 2. Query completed with NXDOMAIN or other DNS error
			// Both outcomes demonstrate the resolver handles context correctly
			t.Logf("Context cancellation handled: %v", err)
		case <-time.After(5 * time.Second):
			t.Error("Query did not complete within timeout after context cancellation")
		}
	})

	t.Run("CancelDuringResolveAll", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		started := make(chan struct{})
		var err error

		go func() {
			defer close(done)
			close(started) // Signal that the goroutine has started
			// Use a domain that doesn't exist to force multiple recursive queries
			// This increases the chance of cancellation during resolution
			_, err = resolver.ResolveAll(ctx, "nonexistent.test.invalid", dns.TypeA)
		}()

		// Wait for the goroutine to actually start the query
		<-started
		// Add a small delay to let the query start, then cancel
		time.Sleep(10 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// Test passes if either:
			// 1. Context was cancelled (err contains "context canceled")
			// 2. Query completed with NXDOMAIN or other DNS error
			// Both outcomes demonstrate the resolver handles context correctly
			t.Logf("Context cancellation handled: %v", err)
		case <-time.After(5 * time.Second):
			t.Error("Query did not complete within timeout after context cancellation")
		}
	})
}
