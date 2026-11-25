package raceinsights

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeyedMutexForgetsKeysOnceReleased(test *testing.T) {
	var keyedLocks keyedMutex
	for raceIndex := range 50 {
		unlock := keyedLocks.lock(fmt.Sprintf("forecast:2026-%02d", raceIndex))
		unlock()
	}
	if trackedKeys := keyedLocks.trackedKeyCount(); trackedKeys != 0 {
		test.Fatalf("released keys must be forgotten, still tracking %d", trackedKeys)
	}
}

func TestKeyedMutexSerializesTheSameKey(test *testing.T) {
	var keyedLocks keyedMutex
	var activeHolders, maximumActiveHolders atomic.Int32
	var waitGroup sync.WaitGroup
	for range 20 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			unlock := keyedLocks.lock("last-race")
			defer unlock()
			currentHolders := activeHolders.Add(1)
			for {
				previousMaximum := maximumActiveHolders.Load()
				if currentHolders <= previousMaximum || maximumActiveHolders.CompareAndSwap(previousMaximum, currentHolders) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			activeHolders.Add(-1)
		}()
	}
	waitGroup.Wait()
	if maximumActiveHolders.Load() != 1 {
		test.Fatalf("one holder at a time expected, saw %d", maximumActiveHolders.Load())
	}
	if trackedKeys := keyedLocks.trackedKeyCount(); trackedKeys != 0 {
		test.Fatalf("key must be forgotten after the last holder, still tracking %d", trackedKeys)
	}
}

func TestKeyedMutexDoesNotBlockOtherKeys(test *testing.T) {
	var keyedLocks keyedMutex
	unlockFirst := keyedLocks.lock("circuit-profile:a:2026")
	defer unlockFirst()

	otherKeyAcquired := make(chan struct{})
	go func() {
		unlockSecond := keyedLocks.lock("circuit-profile:b:2026")
		unlockSecond()
		close(otherKeyAcquired)
	}()
	select {
	case <-otherKeyAcquired:
	case <-time.After(time.Second):
		test.Fatal("a different key must not wait for the held one")
	}
}
