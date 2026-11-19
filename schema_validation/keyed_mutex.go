// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import "sync"

type keyedMutex struct {
	mu    sync.Mutex
	locks map[any]*countedMutex
}

type countedMutex struct {
	mu   sync.Mutex
	refs int
}

func newKeyedMutex() *keyedMutex {
	return &keyedMutex{locks: make(map[any]*countedMutex)}
}

func (k *keyedMutex) lock(key any) {
	k.mu.Lock()
	cm, ok := k.locks[key]
	if !ok {
		cm = &countedMutex{}
		k.locks[key] = cm
	}
	cm.refs++
	k.mu.Unlock()

	cm.mu.Lock()
}

func (k *keyedMutex) unlock(key any) {
	k.mu.Lock()
	defer k.mu.Unlock()

	cm, ok := k.locks[key]
	if !ok {
		return
	}
	cm.mu.Unlock()
	cm.refs--
	if cm.refs == 0 {
		delete(k.locks, key)
	}
}

func (k *keyedMutex) do(key any, fn func()) {
	k.lock(key)
	defer k.unlock(key)
	fn()
}

func (k *keyedMutex) len() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.locks)
}
