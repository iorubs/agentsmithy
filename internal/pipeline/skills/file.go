package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/skills/sandbox"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// fileReadResult is the wire shape for file_read.
type fileReadResult struct {
	Content string `json:"content"`
}

// fileListResult is the wire shape for file_list and file_glob.
type fileListResult struct {
	Paths []string `json:"paths"`
}

// fileWriteResult is the wire shape for file_write.
type fileWriteResult struct {
	Path string `json:"path"`
}

func buildFile(projectRoot string, fk v1.FileSkill) ([]adktool.Tool, map[string]Helper, error) {
	wd := fk.WorkingDir
	if !filepath.IsAbs(wd) {
		wd = filepath.Join(projectRoot, wd)
	}
	sb, err := sandbox.New(wd)
	if err != nil {
		return nil, nil, fmt.Errorf("workingDir %q: %w", fk.WorkingDir, err)
	}

	tools := []adktool.Tool{}
	helpers := map[string]Helper{}

	if fk.Read != nil && fk.Read.Enabled {
		readPaths := fk.Read.Paths
		readT, err := functiontool.New(functiontool.Config{
			Name:        "file_read",
			Description: "Read a UTF-8 text file under the agent's working directory.",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"path": {Type: "string", Description: "Path relative to working directory."},
				},
				Required: []string{"path"},
			},
		}, func(_ adktool.Context, in map[string]any) (fileReadResult, error) {
			p, _ := in["path"].(string)
			b, err := readFile(sb, readPaths, p)
			if err != nil {
				return fileReadResult{}, err
			}
			return fileReadResult{Content: string(b)}, nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("file_read: %w", err)
		}
		tools = append(tools, readT)
		helpers["file_read"] = func(_ context.Context, args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("file_read: expected 1 arg (path), got %d", len(args))
			}
			p, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("file_read: path must be a string")
			}
			b, err := readFile(sb, readPaths, p)
			if err != nil {
				return nil, err
			}
			return string(b), nil
		}

		listT, err := functiontool.New(functiontool.Config{
			Name:        "file_list",
			Description: "List files under a directory inside the agent's working directory.",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"dir": {Type: "string", Description: "Directory relative to working directory. Empty means root."},
				},
			},
		}, func(_ adktool.Context, in map[string]any) (fileListResult, error) {
			dir, _ := in["dir"].(string)
			paths, err := listDir(sb, readPaths, dir)
			if err != nil {
				return fileListResult{}, err
			}
			return fileListResult{Paths: paths}, nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("file_list: %w", err)
		}
		tools = append(tools, listT)
		helpers["file_list"] = func(_ context.Context, args ...any) (any, error) {
			dir := ""
			if len(args) >= 1 {
				if s, ok := args[0].(string); ok {
					dir = s
				}
			}
			return listDir(sb, readPaths, dir)
		}

		globT, err := functiontool.New(functiontool.Config{
			Name:        "file_glob",
			Description: "Find files matching a glob pattern (supports **) under the agent's working directory.",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"pattern": {Type: "string", Description: "Glob pattern relative to working directory."},
				},
				Required: []string{"pattern"},
			},
		}, func(_ adktool.Context, in map[string]any) (fileListResult, error) {
			pat, _ := in["pattern"].(string)
			matches, err := sandbox.Glob(sb.FS(), pat)
			if err != nil {
				return fileListResult{}, fmt.Errorf("glob: %w", err)
			}
			matches = filterByAllow(matches, readPaths)
			return fileListResult{Paths: matches}, nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("file_glob: %w", err)
		}
		tools = append(tools, globT)
		helpers["file_glob"] = func(_ context.Context, args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("file_glob: expected 1 arg (pattern), got %d", len(args))
			}
			pat, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("file_glob: pattern must be a string")
			}
			matches, err := sandbox.Glob(sb.FS(), pat)
			if err != nil {
				return nil, fmt.Errorf("glob: %w", err)
			}
			return filterByAllow(matches, readPaths), nil
		}
	}

	if fk.Write != nil && fk.Write.Enabled {
		writePaths := fk.Write.Paths
		writeT, err := functiontool.New(functiontool.Config{
			Name:        "file_write",
			Description: "Write a UTF-8 text file under the agent's working directory. Creates parent directories.",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"path":    {Type: "string", Description: "Path relative to working directory."},
					"content": {Type: "string", Description: "File content."},
				},
				Required: []string{"path", "content"},
			},
		}, func(_ adktool.Context, in map[string]any) (fileWriteResult, error) {
			p, _ := in["path"].(string)
			c, _ := in["content"].(string)
			abs, err := writeFile(sb, writePaths, p, []byte(c))
			if err != nil {
				return fileWriteResult{}, err
			}
			return fileWriteResult{Path: abs}, nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("file_write: %w", err)
		}
		tools = append(tools, writeT)
		helpers["file_write"] = func(_ context.Context, args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("file_write: expected 2 args (path, content), got %d", len(args))
			}
			p, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("file_write: path must be a string")
			}
			c, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("file_write: content must be a string")
			}
			return writeFile(sb, writePaths, p, []byte(c))
		}

		editT, err := functiontool.New(functiontool.Config{
			Name:        "file_edit",
			Description: "Replace an exact substring in an existing file. The old text must occur exactly once.",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"path": {Type: "string", Description: "Path relative to working directory."},
					"old":  {Type: "string", Description: "Exact substring to replace. Must occur exactly once."},
					"new":  {Type: "string", Description: "Replacement text."},
				},
				Required: []string{"path", "old", "new"},
			},
		}, func(_ adktool.Context, in map[string]any) (fileWriteResult, error) {
			p, _ := in["path"].(string)
			oldS, _ := in["old"].(string)
			newS, _ := in["new"].(string)
			abs, err := editFile(sb, writePaths, p, oldS, newS)
			if err != nil {
				return fileWriteResult{}, err
			}
			return fileWriteResult{Path: abs}, nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("file_edit: %w", err)
		}
		tools = append(tools, editT)
		helpers["file_edit"] = func(_ context.Context, args ...any) (any, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("file_edit: expected 3 args (path, old, new), got %d", len(args))
			}
			p, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("file_edit: path must be a string")
			}
			oldS, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("file_edit: old must be a string")
			}
			newS, ok := args[2].(string)
			if !ok {
				return nil, fmt.Errorf("file_edit: new must be a string")
			}
			return editFile(sb, writePaths, p, oldS, newS)
		}
	}

	return tools, helpers, nil
}

func readFile(sb *sandbox.Sandbox, allow []string, p string) ([]byte, error) {
	if err := checkAllow(allow, p); err != nil {
		return nil, err
	}
	abs, err := sb.Resolve(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

func writeFile(sb *sandbox.Sandbox, allow []string, p string, data []byte) (string, error) {
	if err := checkAllow(allow, p); err != nil {
		return "", err
	}
	abs, err := sb.Resolve(p)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", err
	}
	return abs, nil
}

func editFile(sb *sandbox.Sandbox, allow []string, p, oldS, newS string) (string, error) {
	if oldS == "" {
		return "", errors.New("old must not be empty")
	}
	if err := checkAllow(allow, p); err != nil {
		return "", err
	}
	abs, err := sb.Resolve(p)
	if err != nil {
		return "", err
	}
	cur, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	count := strings.Count(string(cur), oldS)
	if count == 0 {
		return "", errors.New("old not found in file")
	}
	if count > 1 {
		return "", fmt.Errorf("old occurs %d times; must be unique", count)
	}
	updated := strings.Replace(string(cur), oldS, newS, 1)
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}

func listDir(sb *sandbox.Sandbox, allow []string, dir string) ([]string, error) {
	rel := strings.TrimPrefix(dir, "./")
	if rel == "" {
		rel = "."
	}
	if rel != "." {
		if _, err := sb.Resolve(rel); err != nil {
			return nil, err
		}
	}
	entries, err := fs.ReadDir(sb.FS(), rel)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		rp := e.Name()
		if rel != "." {
			rp = path.Join(rel, e.Name())
		}
		if e.IsDir() {
			rp += "/"
		}
		if err := checkAllow(allow, rp); err != nil {
			continue
		}
		out = append(out, rp)
	}
	return out, nil
}

func checkAllow(allow []string, p string) error {
	if len(allow) == 0 {
		return nil
	}
	for _, pat := range allow {
		ok, err := path.Match(pat, p)
		if err == nil && ok {
			return nil
		}
		if strings.Contains(pat, "**") {
			if len(matchDoubleStar(pat, p)) > 0 {
				return nil
			}
		}
	}
	return errors.New("path not in allowed paths")
}

func matchDoubleStar(pat, p string) []string {
	parts := strings.SplitN(pat, "**", 2)
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
	}
	if prefix != "" && !strings.HasPrefix(p, prefix+"/") && p != prefix {
		return nil
	}
	rel := p
	if prefix != "" {
		rel = strings.TrimPrefix(p, prefix+"/")
	}
	if suffix == "" {
		return []string{p}
	}
	n := strings.Count(suffix, "/") + 1
	segs := strings.Split(rel, "/")
	if len(segs) < n {
		return nil
	}
	tail := strings.Join(segs[len(segs)-n:], "/")
	if ok, _ := path.Match(suffix, tail); ok {
		return []string{p}
	}
	return nil
}

func filterByAllow(paths, allow []string) []string {
	if len(allow) == 0 {
		return paths
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if checkAllow(allow, p) == nil {
			out = append(out, p)
		}
	}
	return out
}
