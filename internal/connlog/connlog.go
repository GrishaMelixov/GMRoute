package connlog

import "sync"

type Event struct {
	Domain  string  `json:"domain"`
	Route   string  `json:"route"`
	SrcLat  float64 `json:"src_lat"`
	SrcLng  float64 `json:"src_lng"`
	DstLat  float64 `json:"dst_lat"`
	DstLng  float64 `json:"dst_lng"`
	Country string  `json:"country"`
	Time    int64   `json:"time"`
}

const maxEvents = 200

type Log struct {
	mu     sync.RWMutex
	events []Event
	subsMu sync.Mutex
	subs   []chan Event
}

var Global = &Log{}

func (l *Log) Emit(e Event) {
	l.mu.Lock()
	if len(l.events) >= maxEvents {
		l.events = l.events[1:]
	}
	l.events = append(l.events, e)
	l.mu.Unlock()

	l.subsMu.Lock()
	for _, ch := range l.subs {
		select {
		case ch <- e:
		default:
		}
	}
	l.subsMu.Unlock()
}

func (l *Log) Recent() []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Event, len(l.events))
	copy(result, l.events)
	return result
}

func (l *Log) Subscribe() chan Event {
	ch := make(chan Event, 64)
	l.subsMu.Lock()
	l.subs = append(l.subs, ch)
	l.subsMu.Unlock()
	return ch
}

func (l *Log) Unsubscribe(ch chan Event) {
	l.subsMu.Lock()
	defer l.subsMu.Unlock()
	for i, s := range l.subs {
		if s == ch {
			l.subs = append(l.subs[:i], l.subs[i+1:]...)
			close(ch)
			return
		}
	}
}
