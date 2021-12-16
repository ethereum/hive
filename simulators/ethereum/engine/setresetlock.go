package main

import (
	"sync"
)

// Locking mechanism that contains a "set" value.
// Single thread controls the reset status, others can only lock when they are the first in the queue since last reset.
type SetResetLock struct {
	lock sync.Mutex
	set  bool
}

// Reset the lock so other threads can get it
func (r *SetResetLock) LockReset() {
	r.lock.Lock()
	r.set = false
}

// Lock regardless of the set-reset state
func (r *SetResetLock) Lock() {
	r.lock.Lock()
}

// Only get the lock if it has not been previously set by another thread
func (r *SetResetLock) LockSet() {
	for {
		r.lock.Lock()
		if !r.set {
			// We only get to keep the lock if it's unset
			r.set = true
			break
		}
		// Unlock to try again
		r.lock.Unlock()
	}
}

// Unlock regardless of the set-reset state
func (r *SetResetLock) Unlock() {
	r.lock.Unlock()
}
