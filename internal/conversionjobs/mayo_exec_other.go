//go:build !linux

package conversionjobs

import (
	"os/exec"
	"time"
)

func configureMayoCommand(command *exec.Cmd) {
	// Preserve CommandContext's direct-process cancellation
	// on other operating systems while bounding pipe waits.
	// Process-group termination is implemented for Linux.
	command.WaitDelay = 2 * time.Second
}
