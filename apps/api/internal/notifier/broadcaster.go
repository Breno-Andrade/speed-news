// Package notifier fans out unread-count changes to every connected listener (SSE clients).
package notifier

import "sync"

// UnreadCountChanged is the payload sent to listeners.
type UnreadCountChanged struct {
	UnreadCount int `json:"unreadCount"`
}

// Broadcaster keeps the last value so new listeners receive the current state immediately.
type Broadcaster struct {
	mutex             sync.Mutex
	listenerChannels  map[chan UnreadCountChanged]struct{}
	latestUnreadCount *UnreadCountChanged
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{listenerChannels: make(map[chan UnreadCountChanged]struct{})}
}

// Subscribe returns a channel and a function to stop listening.
func (broadcaster *Broadcaster) Subscribe() (<-chan UnreadCountChanged, func()) {
	listenerChannel := make(chan UnreadCountChanged, 4)

	broadcaster.mutex.Lock()
	broadcaster.listenerChannels[listenerChannel] = struct{}{}
	if broadcaster.latestUnreadCount != nil {
		listenerChannel <- *broadcaster.latestUnreadCount
	}
	broadcaster.mutex.Unlock()

	unsubscribe := func() {
		broadcaster.mutex.Lock()
		defer broadcaster.mutex.Unlock()
		if _, isSubscribed := broadcaster.listenerChannels[listenerChannel]; isSubscribed {
			delete(broadcaster.listenerChannels, listenerChannel)
			close(listenerChannel)
		}
	}
	return listenerChannel, unsubscribe
}

// Publish sends the new count; slow listeners skip intermediate values instead of blocking.
// The newest value always gets through: when a listener's buffer is full, its oldest pending
// value is dropped to make room (dropping the newest one left the tray showing a stale count).
func (broadcaster *Broadcaster) Publish(unreadCountChanged UnreadCountChanged) {
	broadcaster.mutex.Lock()
	defer broadcaster.mutex.Unlock()
	broadcaster.latestUnreadCount = &unreadCountChanged
	for listenerChannel := range broadcaster.listenerChannels {
		for {
			select {
			case listenerChannel <- unreadCountChanged:
			default:
				select {
				case <-listenerChannel:
				default:
				}
				continue
			}
			break
		}
	}
}
