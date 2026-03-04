// Package service provides script execution helpers for claim tests and definition checks.
package service

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// defaultTestTimeout is the default timeout for running test scripts.
const defaultTestTimeout = 30 * time.Second

// executeScript runs an external script and returns whether it passed (exit 0),
// the captured output (stdout+stderr combined), and any execution error.
// The script runs in the given working directory with the specified timeout.
func executeScript(scriptPath, workDir string, timeout time.Duration) (passed bool, output string, err error) {
	if timeout == 0 {
		timeout = defaultTestTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = workDir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	output = buf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return false, output, fmt.Errorf("script timed out after %v", timeout)
	}

	if runErr != nil {
		// Non-zero exit code means test failed (not an execution error)
		if _, ok := runErr.(*exec.ExitError); ok {
			return false, output, nil
		}
		return false, output, fmt.Errorf("failed to execute script: %w", runErr)
	}

	return true, output, nil
}

// executeSympyExpression runs a sympy expression via python3 and returns whether
// the expression evaluated to True (exit 0), the captured output, and any error.
func executeSympyExpression(expression, workDir string, timeout time.Duration) (passed bool, output string, err error) {
	if timeout == 0 {
		timeout = defaultTestTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pyCode := fmt.Sprintf("from sympy import *; result = (%s); print(result); exit(0 if result else 1)", expression)
	cmd := exec.CommandContext(ctx, "python3", "-c", pyCode)
	cmd.Dir = workDir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	output = buf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return false, output, fmt.Errorf("sympy expression timed out after %v", timeout)
	}

	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); ok {
			return false, output, nil
		}
		return false, output, fmt.Errorf("failed to execute python3: %w", runErr)
	}

	return true, output, nil
}
