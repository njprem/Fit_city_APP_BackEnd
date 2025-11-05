package logging

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// LogstashWriter streams log entries to a Logstash TCP input while keeping the
// standard log package non-blocking. It keeps a single TCP connection open and
// silently drops writes while Logstash is unreachable.
type LogstashWriter struct {
	addr          string
	dialTimeout   time.Duration
	writeTimeout  time.Duration
	retryInterval time.Duration

	mu        sync.Mutex
	conn      net.Conn
	nextRetry time.Time
	closed    bool
}

// Option configures a LogstashWriter.
type Option func(*LogstashWriter)

// WithDialTimeout overrides the TCP dial timeout. Defaults to 2 seconds.
func WithDialTimeout(d time.Duration) Option {
	return func(w *LogstashWriter) {
		w.dialTimeout = d
	}
}

// WithWriteTimeout overrides the TCP write timeout. Defaults to 1 second.
func WithWriteTimeout(d time.Duration) Option {
	return func(w *LogstashWriter) {
		w.writeTimeout = d
	}
}

// WithRetryInterval overrides the cool-down window after a failed connect or
// write. Defaults to 5 seconds.
func WithRetryInterval(d time.Duration) Option {
	return func(w *LogstashWriter) {
		w.retryInterval = d
	}
}

// NewLogstashWriter returns a writer that mirrors log output to a Logstash TCP
// input. The returned writer is safe for concurrent use by multiple goroutines.
func NewLogstashWriter(addr string, opts ...Option) (*LogstashWriter, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, errors.New("logstash: empty address")
	}

	w := &LogstashWriter{
		addr:          addr,
		dialTimeout:   2 * time.Second,
		writeTimeout:  time.Second,
		retryInterval: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// Write implements io.Writer. It attempts to forward the payload to Logstash
// while ensuring the caller never blocks on network hiccups. When Logstash is
// down, writes are dropped until the next retry window.
func (w *LogstashWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	data := make([]byte, len(p))
	copy(data, p)
	if data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, io.ErrClosedPipe
	}

	if err := w.ensureConnLocked(); err != nil {
		return len(p), nil
	}

	if w.writeTimeout > 0 {
		_ = w.conn.SetWriteDeadline(time.Now().Add(w.writeTimeout))
	}

	if _, err := w.conn.Write(data); err != nil {
		w.closeConnLocked()
		w.scheduleRetryLocked()
		return len(p), nil
	}

	return len(p), nil
}

// Close tears down the underlying TCP connection.
func (w *LogstashWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	return w.closeConnLocked()
}

func (w *LogstashWriter) ensureConnLocked() error {
	if w.conn != nil {
		return nil
	}

	now := time.Now()
	if !w.nextRetry.IsZero() && now.Before(w.nextRetry) {
		return errRetryCooldown
	}

	conn, err := net.DialTimeout("tcp", w.addr, w.dialTimeout)
	if err != nil {
		w.scheduleRetryLocked()
		return err
	}

	w.conn = conn
	w.nextRetry = time.Time{}
	return nil
}

func (w *LogstashWriter) closeConnLocked() error {
	if w.conn == nil {
		return nil
	}

	err := w.conn.Close()
	w.conn = nil
	return err
}

func (w *LogstashWriter) scheduleRetryLocked() {
	if w.retryInterval <= 0 {
		w.nextRetry = time.Time{}
		return
	}
	w.nextRetry = time.Now().Add(w.retryInterval)
}

var errRetryCooldown = errors.New("logstash: retry cooldown in effect")
