package raceinsights

import "sync"

// keyedMutex hands out one lock per key and forgets a key as soon as nobody holds or waits for
// it. A sync.Map of mutexes would keep one entry per past race forever in a process that runs
// for months.
type keyedMutex struct {
	mutex   sync.Mutex
	entries map[string]*keyedMutexEntry
}

type keyedMutexEntry struct {
	lock sync.Mutex
	// Goroutines holding or waiting for this key; the entry is removed when it drops to zero.
	interestedCount int
}

// lock blocks until the key is free and returns the function that releases it.
func (keyed *keyedMutex) lock(key string) func() {
	keyed.mutex.Lock()
	if keyed.entries == nil {
		keyed.entries = make(map[string]*keyedMutexEntry)
	}
	entry, isKnown := keyed.entries[key]
	if !isKnown {
		entry = &keyedMutexEntry{}
		keyed.entries[key] = entry
	}
	entry.interestedCount++
	keyed.mutex.Unlock()

	entry.lock.Lock()
	return func() {
		entry.lock.Unlock()
		keyed.mutex.Lock()
		entry.interestedCount--
		if entry.interestedCount == 0 {
			delete(keyed.entries, key)
		}
		keyed.mutex.Unlock()
	}
}

// trackedKeyCount is the number of keys currently held or awaited (tests).
func (keyed *keyedMutex) trackedKeyCount() int {
	keyed.mutex.Lock()
	defer keyed.mutex.Unlock()
	return len(keyed.entries)
}
