package optimizer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"app/internal/folders"
)

// toolError is a tool that crashed, exited with an error or ran past its timeout. The image is
// failed and tried again later.
type toolError struct {
	name    string
	timeout time.Duration
	err     error
	stderr  string
}

func (e *toolError) Error() string {
	if e.timeout > 0 {
		return fmt.Sprintf("%s ran longer than %s", e.name, e.timeout)
	}
	msg := fmt.Sprintf("%s: %v", e.name, e.err)
	if e.stderr != "" {
		msg += ": " + e.stderr
	}
	return msg
}

// stderrLimit is how much of a failing tool's error output goes into the image's reason.
const stderrLimit = 300

// runStep runs one program on the work file with a timeout, in its own process group so a timeout
// also kills anything it started. Under nice it runs as `nice -n 10 [ionice -c3] program ...`,
// with ionice only where it exists (Linux).
func runStep(ctx context.Context, s Step, profile folders.Profile, file, dir string, nice bool, timeout time.Duration) error {
	argv := append([]string{s.path}, s.args(profile, file)...)
	if nice {
		argv = append(lowPriority(), argv...)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &toolError{name: s.name, timeout: timeout}
	}
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && slices.Contains(s.unchanged, exit.ExitCode()) {
		return nil
	}
	msg := strings.TrimSpace(stderr.String())
	if len(msg) > stderrLimit {
		msg = msg[:stderrLimit]
	}
	return &toolError{name: s.name, err: err, stderr: msg}
}

func lowPriority() []string {
	var prefix []string
	if nice, err := exec.LookPath("nice"); err == nil {
		prefix = append(prefix, nice, "-n", "10")
	}
	if ionice, err := exec.LookPath("ionice"); err == nil {
		prefix = append(prefix, ionice, "-c3")
	}
	return prefix
}
