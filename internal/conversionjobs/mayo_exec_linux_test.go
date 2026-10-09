//go:build linux

package conversionjobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExecMayoCommandRunnerCompletesNormally(t *testing.T) {
	output, err := (execMayoCommandRunner{}).Run(
		context.Background(),
		"/bin/sh",
		"-c",
		"printf 'ok'",
	)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if string(output) != "ok" {
		t.Fatalf("expected ok, got %q", output)
	}
}

func TestExecMayoCommandRunnerKillsDescendantsOnCancel(
	t *testing.T,
) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)

	go func() {
		_, err := (execMayoCommandRunner{}).Run(
			ctx,
			"/bin/sh",
			"-c",
			`sleep 30 & printf '%s\n' "$!" > "$1"; wait`,
			"sh",
			pidFile,
		)
		result <- err
	}()

	var childPID int
	startDeadline := time.Now().Add(3 * time.Second)

	for childPID == 0 {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, err = strconv.Atoi(
				strings.TrimSpace(string(data)),
			)
			if err != nil || childPID <= 0 {
				cancel()
				t.Fatalf("invalid child PID %q: %v", data, err)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			cancel()
			t.Fatalf("read child PID: %v", err)
		}

		select {
		case err := <-result:
			cancel()
			t.Fatalf("command exited before child started: %v", err)
		default:
		}

		if time.Now().After(startDeadline) {
			cancel()
			t.Fatal("child process did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Safety cleanup if the subprocess-group assertion fails.
	t.Cleanup(func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
	})

	cancel()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected canceled command to fail")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("command did not return after cancellation")
	}

	exitDeadline := time.Now().Add(2 * time.Second)

	for {
		running, err := linuxProcessRunning(childPID)
		if err != nil {
			t.Fatalf("check descendant state: %v", err)
		}
		if !running {
			return
		}

		if time.Now().After(exitDeadline) {
			t.Fatalf(
				"descendant PID %d survived command cancellation",
				childPID,
			)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func linuxProcessRunning(pid int) (bool, error) {
	path := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// /proc/<pid>/stat contains a parenthesized command name.
	// The first field after its closing parenthesis is state.
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return false, fmt.Errorf("invalid process stat for PID %d", pid)
	}

	fields := strings.Fields(string(data)[end+1:])
	if len(fields) == 0 {
		return false, fmt.Errorf("missing process state for PID %d", pid)
	}

	// A zombie or dead process is no longer executing.
	return fields[0] != "Z" &&
		fields[0] != "X" &&
		fields[0] != "x", nil
}
