package sandbox

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Glob returns relative paths matching pattern against the given fs.FS.
// Supports `**` patterns via directory walk; otherwise delegates to
// fs.Glob.
func Glob(fsys fs.FS, pattern string) ([]string, error) {
	if strings.Contains(pattern, "**") {
		return expandDoubleStar(fsys, pattern), nil
	}
	return fs.Glob(fsys, pattern)
}

func expandDoubleStar(fsys fs.FS, pattern string) []string {
	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
	}

	walkRoot := "."
	if prefix != "" {
		walkRoot = prefix
	}

	var matches []string
	_ = fs.WalkDir(fsys, walkRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if suffix == "" {
			matches = append(matches, path)
			return nil
		}
		rel := path
		if walkRoot != "." {
			rel = strings.TrimPrefix(path, walkRoot+"/")
		}
		n := strings.Count(suffix, "/") + 1
		segs := strings.Split(rel, "/")
		if len(segs) >= n {
			tail := strings.Join(segs[len(segs)-n:], "/")
			if ok, _ := filepath.Match(suffix, tail); ok {
				matches = append(matches, path)
			}
		}
		return nil
	})
	return matches
}
