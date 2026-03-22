package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"mebot/types"
)

// ---- run_command ----

type RunCommandTool struct{}

func (t *RunCommandTool) Name() string        { return "run_command" }
func (t *RunCommandTool) Description() string { return "Execute a shell command on the host machine" }

func (t *RunCommandTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "run_command",
		Description: "Execute a shell command and return stdout/stderr. Use this for running scripts, setting up cron jobs, checking system info, file operations, etc. Commands run on the host machine directly.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"command": {
					Type:        "STRING",
					Description: "The shell command to execute (e.g. 'echo hello', 'ls -la', 'cat /etc/crontab')",
				},
				"working_dir": {
					Type:        "STRING",
					Description: "Optional working directory for the command. Defaults to current directory.",
				},
			},
			Required: []string{"command"},
		},
	}
}

func (t *RunCommandTool) Execute(args map[string]any) types.ToolResult {
	command, _ := args["command"].(string)
	if command == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: command"}
	}

	workDir, _ := args["working_dir"].(string)

	// Create the command based on OS
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-Command", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}

	if workDir != "" {
		cmd.Dir = workDir
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		output := stdout.String()
		errOutput := stderr.String()

		// Truncate if too long
		if len(output) > 4000 {
			output = output[:4000] + "\n... (truncated)"
		}
		if len(errOutput) > 1000 {
			errOutput = errOutput[:1000] + "\n... (truncated)"
		}

		var result strings.Builder
		if output != "" {
			result.WriteString(output)
		}
		if errOutput != "" {
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString("[STDERR]\n")
			result.WriteString(errOutput)
		}

		if err != nil {
			return types.ToolResult{
				Status: "error",
				Output: fmt.Sprintf("Command failed: %v\n%s", err, result.String()),
			}
		}

		if result.Len() == 0 {
			result.WriteString("(command completed with no output)")
		}

		return types.ToolResult{
			Status: "success",
			Output: result.String(),
		}

	case <-time.After(60 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return types.ToolResult{
			Status: "error",
			Output: "Command timed out after 60 seconds",
		}
	}
}
