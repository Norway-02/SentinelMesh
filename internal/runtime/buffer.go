package runtime

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// LogLine represents a single timestamped line of output.
type LogLine struct {
	Timestamp time.Time
	Content   []byte
}

// LogBuffer is a thread-safe ring buffer for capturing stdout/stderr streams
// with real-time subscription capabilities for log tailing.
type LogBuffer struct {
	mu          sync.RWMutex
	lines       []LogLine
	maxCapacity int
	closed      bool
	subscribers map[chan []byte]struct{}
}

// NewLogBuffer constructs a LogBuffer with a max line capacity.
func NewLogBuffer(maxCapacity int) *LogBuffer {
	if maxCapacity <= 0 {
		maxCapacity = 5000
	}
	return &LogBuffer{
		lines:       make([]LogLine, 0, 128),
		maxCapacity: maxCapacity,
		subscribers: make(map[chan []byte]struct{}),
	}
}

// Write appends raw bytes to the log buffer, splitting by newline.
func (b *LogBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return 0, io.ErrClosedPipe
	}

	now := time.Now()
	chunks := bytes.Split(p, []byte("\n"))
	for i, chunk := range chunks {
		if len(chunk) == 0 && i == len(chunks)-1 {
			// Trailing newline
			continue
		}

		lineCopy := make([]byte, len(chunk))
		copy(lineCopy, chunk)

		entry := LogLine{
			Timestamp: now,
			Content:   lineCopy,
		}

		if len(b.lines) >= b.maxCapacity {
			// Shift ring buffer
			b.lines = b.lines[1:]
		}
		b.lines = append(b.lines, entry)

		// Broadcast to active live subscribers
		for ch := range b.subscribers {
			select {
			case ch <- append(lineCopy, '\n'):
			default:
				// Skip if channel is full to prevent deadlocking writers
			}
		}
	}

	return len(p), nil
}

// Close marks the buffer as closed and signals all active live streaming subscribers.
func (b *LogBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}
	b.closed = true

	for ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, ch)
	}

	return nil
}

// StreamLogReader implements io.ReadCloser for tailing logs.
type StreamLogReader struct {
	buffer     *LogBuffer
	ch         chan []byte
	currentBuf *bytes.Buffer
	closed     bool
	mu         sync.Mutex
}

// NewReader returns an io.ReadCloser that outputs past logs (tail) and optionally
// follows live output until buffer is closed or reader is closed.
func (b *LogBuffer) NewReader(tailLines int, follow bool) io.ReadCloser {
	b.mu.Lock()
	defer b.mu.Unlock()

	var pastBytes bytes.Buffer
	total := len(b.lines)
	start := 0
	if tailLines > 0 && tailLines < total {
		start = total - tailLines
	}

	for i := start; i < total; i++ {
		pastBytes.Write(b.lines[i].Content)
		pastBytes.WriteByte('\n')
	}

	if !follow || b.closed {
		return io.NopCloser(&pastBytes)
	}

	ch := make(chan []byte, 100)
	b.subscribers[ch] = struct{}{}

	return &StreamLogReader{
		buffer:     b,
		ch:         ch,
		currentBuf: &pastBytes,
	}
}

// Read reads available data from past logs or incoming streaming channel.
func (r *StreamLogReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, io.EOF
	}

	if r.currentBuf != nil && r.currentBuf.Len() > 0 {
		return r.currentBuf.Read(p)
	}

	chunk, ok := <-r.ch
	if !ok {
		return 0, io.EOF
	}

	r.currentBuf = bytes.NewBuffer(chunk)
	return r.currentBuf.Read(p)
}

// Close cancels the log subscription.
func (r *StreamLogReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	r.buffer.mu.Lock()
	delete(r.buffer.subscribers, r.ch)
	r.buffer.mu.Unlock()

	return nil
}

// GetLines returns a snapshot copy of all recorded log lines.
func (b *LogBuffer) GetLines() []LogLine {
	b.mu.RLock()
	defer b.mu.RUnlock()

	res := make([]LogLine, len(b.lines))
	copy(res, b.lines)
	return res
}
