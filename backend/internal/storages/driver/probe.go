package driver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Step names the part of a connection test that failed.
type Step string

const (
	StepConnect Step = "connect"
	StepWrite   Step = "write"
	StepRead    Step = "read"
	StepDelete  Step = "delete"
)

// ProbeError is a failed connection test: the step and what went wrong there.
type ProbeError struct {
	Step Step
	Err  error
}

func (e *ProbeError) Error() string { return fmt.Sprintf("%s: %v", e.Step, e.Err) }
func (e *ProbeError) Unwrap() error { return e.Err }

var errProbeMismatch = errors.New("the object read back differs from the one written")

// Probe checks that the storage can hold a backup: it writes a small object, reads it back,
// compares, and deletes it. It returns nil, or a *ProbeError for the first step that failed.
func Probe(ctx context.Context, d Driver) error {
	key, content := probeObject()

	if err := d.Put(ctx, key, bytes.NewReader(content)); err != nil {
		return &ProbeError{Step: StepWrite, Err: err}
	}

	if err := readBack(ctx, d, key, content); err != nil {
		_ = d.Delete(ctx, key)
		return &ProbeError{Step: StepRead, Err: err}
	}

	if err := d.Delete(ctx, key); err != nil {
		return &ProbeError{Step: StepDelete, Err: err}
	}
	return nil
}

func probeObject() (string, []byte) {
	content := make([]byte, 32)
	rand.Read(content)
	return ".gogobackup-probe-" + hex.EncodeToString(content[:8]), content
}

func readBack(ctx context.Context, d Driver, key string, want []byte) error {
	r, err := d.Open(ctx, key)
	if err != nil {
		return err
	}
	defer r.Close()

	got, err := io.ReadAll(io.LimitReader(r, int64(len(want))+1))
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return errProbeMismatch
	}
	return nil
}
