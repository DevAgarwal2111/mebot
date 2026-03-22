package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mebot/types"
)

// projectRoot is the base directory of the MeBot project.
// Tools will restrict file operations to this directory.
var projectRoot string

func init() {
	// Determine project root from the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	projectRoot = cwd
}

// isPathSafe checks that the resolved path is within the project root
// or within common safe directories.
func isPathSafe(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	// Allow project directory and temp
	if strings.HasPrefix(absPath, projectRoot) {
		return true
	}
	// Allow temp directory
	if strings.HasPrefix(absPath, os.TempDir()) {
		return true
	}
	return false
}

// ---- read_file ----

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read the contents of a file" }

func (t *ReadFileTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "read_file",
		Description: "Read the full contents of a file. Use this to inspect source code, config files, or any text file.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"path": {
					Type:        "STRING",
					Description: "Path to the file to read (relative to project root or absolute)",
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *ReadFileTool) Execute(args map[string]any) types.ToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: path"}
	}

	// Resolve relative paths against project root
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}

	if !isPathSafe(path) {
		return types.ToolResult{Status: "error", Output: "Access denied: path is outside the project directory"}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to read file: %v", err)}
	}

	// Truncate very large files
	content := string(data)
	if len(content) > 50000 {
		content = content[:50000] + "\n\n... (truncated, file too large)"
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("File: %s (%d bytes)\n\n%s", path, len(data), content),
	}
}

// ---- write_file ----

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write or overwrite a file with new content" }

func (t *WriteFileTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "write_file",
		Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Use this to create new source files, modify existing code, or update configs. Parent directories are created automatically.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"path": {
					Type:        "STRING",
					Description: "Path to the file to write (relative to project root or absolute)",
				},
				"content": {
					Type:        "STRING",
					Description: "The full content to write to the file",
				},
			},
			Required: []string{"path", "content"},
		},
	}
}

func (t *WriteFileTool) Execute(args map[string]any) types.ToolResult {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	if path == "" || content == "" {
		return types.ToolResult{Status: "error", Output: "Missing required arguments: path and content"}
	}

	// Resolve relative paths
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}

	if !isPathSafe(path) {
		return types.ToolResult{Status: "error", Output: "Access denied: path is outside the project directory"}
	}

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to create directories: %v", err)}
	}

	// Check if file exists (for reporting)
	_, existErr := os.Stat(path)
	action := "Created"
	if existErr == nil {
		action = "Updated"
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to write file: %v", err)}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("%s file: %s (%d bytes written). If this is a .go file, 'air' will auto-reload the server.", action, path, len(content)),
	}
}

// ---- list_directory ----

type ListDirectoryTool struct{}

func (t *ListDirectoryTool) Name() string        { return "list_directory" }
func (t *ListDirectoryTool) Description() string { return "List files and directories at a path" }

func (t *ListDirectoryTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "list_directory",
		Description: "List all files and subdirectories at a given path. Useful for understanding project structure.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"path": {
					Type:        "STRING",
					Description: "Directory path to list (relative to project root or absolute). Use '.' for project root.",
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *ListDirectoryTool) Execute(args map[string]any) types.ToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}

	if !isPathSafe(path) {
		return types.ToolResult{Status: "error", Output: "Access denied: path is outside the project directory"}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to list directory: %v", err)}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory: %s\n\n", path))

	for _, entry := range entries {
		if entry.IsDir() {
			sb.WriteString(fmt.Sprintf("  📁 %s/\n", entry.Name()))
		} else {
			info, _ := entry.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			sb.WriteString(fmt.Sprintf("  📄 %s (%d bytes)\n", entry.Name(), size))
		}
	}

	return types.ToolResult{Status: "success", Output: sb.String()}
}
