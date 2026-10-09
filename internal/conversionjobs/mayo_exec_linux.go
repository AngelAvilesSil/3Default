//go:build linux

package conversionjobs

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configureMayoCommand(command *exec.Cmd) {
	// Keep the converter and its ordinary descendants in a
	// process group separate from the Go API process.
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// CommandContext normally kills only its direct child.
	// Terminate the process group to include descendants that
	// have not detached into separate process groups.
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}

		err := syscall.Kill(
			-command.Process.Pid,
			syscall.SIGKILL,
		)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}

		return err
	}

	// Bound waiting for output pipes left open by descendants.
	command.WaitDelay = 2 * time.Second
}
