package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrNotInteractive indicates stdin is not a terminal.
var ErrNotInteractive = errors.New("stdin is not a terminal")

// RequireInteractiveStdin checks if stdin is a terminal.
// Returns an error with guidance to use --yes flag if stdin is not interactive.
// The command parameter is used in the error message (e.g., "refute", "archive").
func RequireInteractiveStdin(stdin *os.File, command string) error {
	stat, err := stdin.Stat()
	if err != nil {
		return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm %s in non-interactive mode", command)
	}

	mode := stat.Mode()
	isTerminal := (mode & os.ModeCharDevice) != 0
	isPipe := (mode & os.ModeNamedPipe) != 0

	if !isTerminal || isPipe {
		return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm %s in non-interactive mode", command)
	}

	return nil
}

// ConfirmAction prompts the user for confirmation of a destructive action.
// If skipConfirm is true, returns true immediately without prompting.
// If stdin is not a terminal, returns ErrNotInteractive.
//
// The action parameter should describe what's being confirmed
// (e.g., "refute node 1.2.3", "archive node 1").
//
// Returns:
//   - (true, nil) if user confirms or skipConfirm is true
//   - (false, nil) if user declines
//   - (false, error) if stdin is not interactive or read fails
func ConfirmAction(out io.Writer, action string, skipConfirm bool) (bool, error) {
	if skipConfirm {
		return true, nil
	}

	// Check if stdin is a terminal
	if err := RequireInteractiveStdin(os.Stdin, action); err != nil {
		return false, fmt.Errorf("%w: %v", ErrNotInteractive, err)
	}

	return confirmFromReader(out, os.Stdin, action)
}

// ConfirmActionWithReader is like ConfirmAction but accepts an io.Reader for testing.
// It does not check if the reader is a terminal - use this only for tests.
func ConfirmActionWithReader(out io.Writer, in io.Reader, action string, skipConfirm bool) (bool, error) {
	if skipConfirm {
		return true, nil
	}
	return confirmFromReader(out, in, action)
}

func confirmFromReader(out io.Writer, in io.Reader, action string) (bool, error) {
	fmt.Fprintf(out, "Are you sure you want to %s? [y/N]: ", action)

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil {
		// EOF or other error means non-interactive context
		return false, ErrNotInteractive
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}
