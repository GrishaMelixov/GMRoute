package metrics

import (
	"sync/atomic"
)

type Metrics struct {
	ActiveConns  atomic.Int64
	TotalConns   atomic.Int64
	DirectConns  atomic.Int64
	UpstreamConn atomic.Int64
	Errors       atomic.Int64
}

var Global = &Metrics{}

func (m *Metrics) ConnOpened(upstream bool) {
	m.ActiveConns.Add(1)
	m.TotalConns.Add(1)
	if upstream {
		m.UpstreamConn.Add(1)
	} else {
		m.DirectConns.Add(1)
	}
}

func (m *Metrics) ConnClosed() {
	m.ActiveConns.Add(-1)
}

func (m *Metrics) Error() {
	m.Errors.Add(1)
}
