package notifier

import (
	"sync"
	"testing"
	"time"
)

func receiveWithin(test *testing.T, listenerChannel <-chan UnreadCountChanged) UnreadCountChanged {
	test.Helper()
	select {
	case unreadCountChanged := <-listenerChannel:
		return unreadCountChanged
	case <-time.After(time.Second):
		test.Fatal("no event received")
		return UnreadCountChanged{}
	}
}

func TestNewListenerReceivesTheLatestCountImmediately(test *testing.T) {
	broadcaster := NewBroadcaster()
	broadcaster.Publish(UnreadCountChanged{UnreadCount: 7})

	listenerChannel, unsubscribe := broadcaster.Subscribe()
	defer unsubscribe()
	if received := receiveWithin(test, listenerChannel); received.UnreadCount != 7 {
		test.Errorf("a late subscriber must get the current count, got %d", received.UnreadCount)
	}
}

func TestEveryListenerReceivesPublishedCounts(test *testing.T) {
	broadcaster := NewBroadcaster()
	firstChannel, unsubscribeFirst := broadcaster.Subscribe()
	defer unsubscribeFirst()
	secondChannel, unsubscribeSecond := broadcaster.Subscribe()
	defer unsubscribeSecond()

	broadcaster.Publish(UnreadCountChanged{UnreadCount: 3})
	if receiveWithin(test, firstChannel).UnreadCount != 3 || receiveWithin(test, secondChannel).UnreadCount != 3 {
		test.Error("every listener must receive the count")
	}
}

func TestSlowListenerNeverBlocksPublishers(test *testing.T) {
	broadcaster := NewBroadcaster()
	_, unsubscribe := broadcaster.Subscribe() // never read
	defer unsubscribe()

	publishFinished := make(chan struct{})
	go func() {
		for publishIndex := 0; publishIndex < 100; publishIndex++ {
			broadcaster.Publish(UnreadCountChanged{UnreadCount: publishIndex})
		}
		close(publishFinished)
	}()
	select {
	case <-publishFinished:
	case <-time.After(time.Second):
		test.Fatal("a listener that does not read must not block Publish")
	}
}

func TestUnsubscribeClosesTheChannelAndIsIdempotent(test *testing.T) {
	broadcaster := NewBroadcaster()
	listenerChannel, unsubscribe := broadcaster.Subscribe()
	unsubscribe()
	unsubscribe()

	if _, isOpen := <-listenerChannel; isOpen {
		test.Error("the channel must be closed after unsubscribing")
	}
	broadcaster.Publish(UnreadCountChanged{UnreadCount: 1}) // must not panic on a closed channel
}

func TestConcurrentSubscribeAndPublish(test *testing.T) {
	broadcaster := NewBroadcaster()
	var waitGroup sync.WaitGroup
	for workerIndex := 0; workerIndex < 20; workerIndex++ {
		waitGroup.Add(2)
		go func() {
			defer waitGroup.Done()
			_, unsubscribe := broadcaster.Subscribe()
			unsubscribe()
		}()
		go func(publishedCount int) {
			defer waitGroup.Done()
			broadcaster.Publish(UnreadCountChanged{UnreadCount: publishedCount})
		}(workerIndex)
	}
	waitGroup.Wait()
}
