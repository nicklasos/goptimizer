// Package runlog is the log of one backup run: timestamped lines, capped in size, safe to
// write from the dump, its tools and the uploads at once and to read while they write.
package runlog

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MaxBytes is how much of a log is kept. A tool that prints without end (a verbose dump of a
// huge database) keeps only its latest output, which is where the error is.
const MaxBytes = 1 << 20

// slack lets the buffer grow past MaxBytes before it is cut, so a full log is not copied for
// every new line.
const slack = MaxBytes / 4

type Log struct {
	mu      sync.Mutex
	buf     []byte
	dropped bool
	version uint64
	now     func() time.Time
}

func New() *Log {
	return &Log{now: time.Now}
}

// Printf adds one line. A message with several lines is kept as several lines.
func (l *Log) Printf(format string, args ...any) {
	l.add("", fmt.Sprintf(format, args...))
}

// Warnf adds a line marked as a warning: something the run went past without failing.
func (l *Log) Warnf(format string, args ...any) {
	l.add("warning: ", fmt.Sprintf(format, args...))
}

func (l *Log) add(prefix, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	stamp := l.now().UTC().Format("15:04:05 ")
	for _, line := range strings.Split(strings.TrimRight(message, "\n"), "\n") {
		l.buf = append(l.buf, stamp...)
		l.buf = append(l.buf, prefix...)
		l.buf = append(l.buf, line...)
		l.buf = append(l.buf, '\n')
	}
	l.version++
	if len(l.buf) > MaxBytes+slack {
		l.buf = append([]byte(nil), trim(l.buf)...)
		l.dropped = true
	}
}

// trim is the end of buf that fits in MaxBytes, starting at a line.
func trim(buf []byte) []byte {
	if len(buf) <= MaxBytes {
		return buf
	}
	cut := buf[len(buf)-MaxBytes:]
	if i := bytes.IndexByte(cut, '\n'); i >= 0 {
		cut = cut[i+1:]
	}
	return cut
}

const droppedNote = "[earlier lines were dropped: the log keeps its last 1 MB]\n"

// String is the log as stored: at most MaxBytes of the latest lines, after a note when earlier
// lines were dropped.
func (l *Log) String() string {
	text, _ := l.Snapshot()
	return text
}

// Snapshot is String with a version that changes on every write, so a writer to the database
// can skip a log that has not changed.
func (l *Log) Snapshot() (string, uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := trim(l.buf)
	if l.dropped || len(kept) < len(l.buf) {
		return droppedNote + string(kept), l.version
	}
	return string(kept), l.version
}

// Tail is the last n lines.
func (l *Log) Tail(n int) string {
	lines := strings.Split(strings.TrimRight(l.String(), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// Writer adds what a program prints, a line at a time, each after prefix unless it starts with
// it already, as pg_restore's do. Close adds a last line that did not end in a newline.
func (l *Log) Writer(prefix string) *LineWriter {
	return &LineWriter{log: l, prefix: prefix}
}

type LineWriter struct {
	log     *Log
	prefix  string
	mu      sync.Mutex
	pending []byte
}

func (w *LineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		w.line(strings.TrimRight(string(w.pending[:i]), "\r"))
		w.pending = w.pending[i+1:]
	}
	if len(w.pending) > MaxBytes {
		w.line(string(w.pending))
		w.pending = nil
	}
	return len(p), nil
}

func (w *LineWriter) line(text string) {
	if strings.HasPrefix(text, w.prefix) {
		w.log.add("", text)
		return
	}
	w.log.add(w.prefix, text)
}

func (w *LineWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		w.line(string(w.pending))
		w.pending = nil
	}
	return nil
}
