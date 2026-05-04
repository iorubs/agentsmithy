// Package sandbox restricts file operations to a project root.
//
// Resolve and ValidateFilePath check that paths (after symlink
// evaluation) stay under the configured root. Copied from
// mcpsmithy/internal/tools/sandbox.go; will be deduped when a
// shared module is extracted.
package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox restricts file operations to a project root.
type Sandbox struct {
	root string
	fsys fs.FS
}

// New creates a sandbox rooted at dir. The directory need not exist;
// resolution falls back to the lexical absolute path when the target
// is missing.
func New(dir string) (*Sandbox, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("abs: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			resolved = abs
		} else {
			return nil, err
		}
	}
	return &Sandbox{root: resolved, fsys: os.DirFS(resolved)}, nil
}

// Root returns the resolved sandbox root.
func (s *Sandbox) Root() string { return s.root }

// FS returns an fs.FS rooted at the sandbox.
func (s *Sandbox) FS() fs.FS { return s.fsys }

// Resolve maps path (absolute or relative to the root) to an absolute
// path inside the sandbox. Returns an error if the resolved path
// (post symlink eval) escapes the root.
func (s *Sandbox) Resolve(path string) (string, error) {
	var abs string
	if filepath.IsAbs(path) {
		abs = path
	} else {
		abs = filepath.Join(s.root, path)
	}
	abs = filepath.Clean(abs)

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			resolved = abs
		} else {
			return "", err
		}
	}

	if resolved != s.root && !strings.HasPrefix(resolved, s.root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside project root %q", path, s.root)
	}
	return resolved, nil
}
