package runtime

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

func TestLogBuffer_WriteAndReadSnapshot(t *testing.T) {
	buf := NewLogBuffer(10)

	_, err := buf.Write([]byte("line 1\nline 2\nline 3\n"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	lines := buf.GetLines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if string(lines[0].Content) != "line 1" {
		t.Errorf("expected 'line 1', got '%s'", string(lines[0].Content))
	}
	if string(lines[2].Content) != "line 3" {
		t.Errorf("expected 'line 3', got '%s'", string(lines[2].Content))
	}
}

func TestLogBuffer_RingCapacity(t *testing.T) {
	buf := NewLogBuffer(3) // capacity 3

	for i := 1; i <= 5; i++ {
		_, _ = buf.Write([]byte("message\n"))
	}

	lines := buf.GetLines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (capped), got %d", len(lines))
	}
}

func TestLogBuffer_LiveStreamReader(t *testing.T) {
	buf := NewLogBuffer(100)
	_, _ = buf.Write([]byte("initial line\n"))

	reader := buf.NewReader(0, true) // follow = true
	defer reader.Close()

	var received bytes.Buffer
	doneCh := make(chan struct{})

	go func() {
		var chunk [128]byte
		for {
			n, err := reader.Read(chunk[:])
			if n > 0 {
				received.Write(chunk[:n])
			}
			if err == io.EOF {
				break
			}
		}
		close(doneCh)
	}()

	// Write more after stream started
	time.Sleep(20 * time.Millisecond)
	_, _ = buf.Write([]byte("live line 1\nlive line 2\n"))
	time.Sleep(20 * time.Millisecond)
	_ = buf.Close()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stream reader to finish")
	}

	out := received.String()
	if !bytes.Contains(received.Bytes(), []byte("initial line\n")) {
		t.Errorf("missing initial line in output: %s", out)
	}
	if !bytes.Contains(received.Bytes(), []byte("live line 1\n")) {
		t.Errorf("missing live line 1 in output: %s", out)
	}
	if !bytes.Contains(received.Bytes(), []byte("live line 2\n")) {
		t.Errorf("missing live line 2 in output: %s", out)
	}
}

func TestLogBuffer_ConcurrentWrites(t *testing.T) {
	buf := NewLogBuffer(1000)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = buf.Write([]byte("concurrent line\n"))
			}
		}(i)
	}

	wg.Wait()
	lines := buf.GetLines()
	if len(lines) != 200 {
		t.Errorf("expected 200 lines from concurrent writers, got %d", len(lines))
	}
}
