package ingestion

import "sync/atomic"

type streamCounters struct {
	received  atomic.Int64
	dropped   atomic.Int64
	errors    atomic.Int64
	reconnect atomic.Int64
	sequence  atomic.Int64
}

func (c *streamCounters) snapshot() StreamMetrics {
	return StreamMetrics{
		ReceivedChunks: c.received.Load(),
		DroppedChunks:  c.dropped.Load(),
		ErrorCount:     c.errors.Load(),
		ReconnectCount: c.reconnect.Load(),
		LastSequence:   c.sequence.Load(),
	}
}
