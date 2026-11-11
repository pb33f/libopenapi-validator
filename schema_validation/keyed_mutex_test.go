// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"sync"
	"testing"
	"time"

	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
)

func TestKeyedMutex_SameKeyIsMutuallyExclusive(t *testing.T) {
	km := newKeyedMutex()

	const goroutines = 50
	const perGoroutine = 200
	counters := map[string]*int{"x": new(int), "y": new(int)}

	var wg sync.WaitGroup
	for i := range goroutines {
		key := "x"
		if i%2 == 1 {
			key = "y"
		}
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			for range perGoroutine {
				km.do(key, func() {
					*counters[key]++
				})
			}
		}(key)
	}
	wg.Wait()

	assert.Equal(t, (goroutines/2)*perGoroutine, *counters["x"])
	assert.Equal(t, (goroutines/2)*perGoroutine, *counters["y"])
	assert.Equal(t, 0, km.len(), "every key must be reclaimed once released")
}

func TestKeyedMutex_SameKeyBlocksConcurrentHolder(t *testing.T) {
	km := newKeyedMutex()
	const key = "doc"

	km.lock(key)

	entered := make(chan struct{})
	released := make(chan struct{})
	go func() {
		km.lock(key)
		close(entered)
		km.unlock(key)
		close(released)
	}()

	select {
	case <-entered:
		km.unlock(key)
		t.Fatal("second holder entered the critical section while the key was locked")
	case <-time.After(200 * time.Millisecond):
	}

	assert.Equal(t, 1, km.len())

	km.unlock(key)

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("second holder never acquired the key after it was released")
	}

	<-released
	assert.Equal(t, 0, km.len())
}

func TestKeyedMutex_DifferentKeysRunConcurrently(t *testing.T) {
	km := newKeyedMutex()

	entered := make(chan struct{}, 2)
	release := make(chan struct{})

	for _, key := range []string{"a", "b"} {
		go func(key string) {
			km.do(key, func() {
				entered <- struct{}{}
				<-release
			})
		}(key)
	}

	for range 2 {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("different keys blocked each other - lock is not scoped per key")
		}
	}
	close(release)
}

func TestKeyedMutex_NoLeakUnderChurn(t *testing.T) {
	km := newKeyedMutex()

	var wg sync.WaitGroup
	for g := range 64 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range 1000 {
				km.do(g*1000+i, func() {})
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, 0, km.len(), "keyed mutex leaked entries after all work drained")
}

func TestKeyedMutex_DoReleasesLockOnPanic(t *testing.T) {
	km := newKeyedMutex()
	const key = "boom"

	require.Panics(t, func() {
		km.do(key, func() { panic("kaboom") })
	})

	assert.Equal(t, 0, km.len(), "panicking key must still be reclaimed")

	done := make(chan struct{})
	go func() {
		km.do(key, func() {})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("re-locking a key after a panic deadlocked")
	}
}

func TestKeyedMutex_UnlockWithoutLockIsNoOp(t *testing.T) {
	km := newKeyedMutex()
	assert.NotPanics(t, func() { km.unlock("never-locked") })
	assert.Equal(t, 0, km.len())
}
