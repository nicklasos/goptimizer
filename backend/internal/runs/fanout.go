package runs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"sync"
)

// Destination is one place a dump is uploaded to. Put reads the archive until EOF and
// returns nil only when it is stored.
type Destination struct {
	Name string
	Put  func(ctx context.Context, r io.Reader) error
}

// Upload is how one destination did: Err is nil when it stored the whole archive.
type Upload struct {
	Name string
	Err  error
}

// FanOut sends one stream to every destination at once, each through its own pipe into its
// own Put, while counting and hashing it. A destination that fails is dropped and the others
// carry on; Write fails only when none is left. Write is for one goroutine, as io.Copy uses it.
type FanOut struct {
	targets []*target
	hash    hash.Hash
	size    int64
	wg      sync.WaitGroup
}

type target struct {
	name string
	pipe *io.PipeWriter
	// err is the reason the destination was dropped while writing; nil while it takes data.
	err error
	// putErr is what Put returned, read after the uploads are waited for.
	putErr error
}

var ErrNoDestination = errors.New("every storage failed")

// errStoppedReading is what a write meets when Put returned without reading to the end.
var errStoppedReading = errors.New("the upload stopped before the end of the archive")

func NewFanOut(ctx context.Context, destinations []Destination) *FanOut {
	f := &FanOut{hash: sha256.New()}
	for _, d := range destinations {
		reader, writer := io.Pipe()
		t := &target{name: d.Name, pipe: writer}
		f.targets = append(f.targets, t)
		f.wg.Add(1)
		go func(put func(context.Context, io.Reader) error) {
			defer f.wg.Done()
			t.putErr = put(ctx, reader)
			reader.CloseWithError(errStoppedReading)
		}(d.Put)
	}
	return f
}

func (f *FanOut) Write(p []byte) (int, error) {
	live := 0
	for _, t := range f.targets {
		if t.err != nil {
			continue
		}
		if _, err := t.pipe.Write(p); err != nil {
			t.err = err
			continue
		}
		live++
	}
	if live == 0 {
		return 0, ErrNoDestination
	}
	f.hash.Write(p)
	f.size += int64(len(p))
	return len(p), nil
}

// Close ends the stream and waits for every upload. Call it when the dump succeeded.
func (f *FanOut) Close() []Upload {
	return f.finish(nil)
}

// Abort ends the stream with err, so every upload fails instead of storing a partial archive,
// and waits for them.
func (f *FanOut) Abort(err error) []Upload {
	return f.finish(err)
}

func (f *FanOut) finish(cause error) []Upload {
	for _, t := range f.targets {
		t.pipe.CloseWithError(cause)
	}
	f.wg.Wait()
	uploads := make([]Upload, len(f.targets))
	for i, t := range f.targets {
		uploads[i] = Upload{Name: t.name, Err: t.result(cause)}
	}
	return uploads
}

func (t *target) result(cause error) error {
	switch {
	case t.putErr != nil:
		return t.putErr
	case t.err != nil:
		return t.err
	case cause != nil:
		return fmt.Errorf("upload abandoned: %w", cause)
	}
	return nil
}

// Live is how many destinations still take data.
func (f *FanOut) Live() int {
	live := 0
	for _, t := range f.targets {
		if t.err == nil {
			live++
		}
	}
	return live
}

// Size is the number of bytes written to the destinations.
func (f *FanOut) Size() int64 { return f.size }

// SHA256 is the hex digest of everything written.
func (f *FanOut) SHA256() string { return hex.EncodeToString(f.hash.Sum(nil)) }
