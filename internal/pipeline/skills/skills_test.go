package skills

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
)

func TestBuild_Shell_Echo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires /bin/echo")
	}
	tmp := t.TempDir()
	built, err := Build("agent", "", v1.Skills{
		Shell: map[string]v1.ShellSkill{
			"say_hi": {
				Command:    []string{"/bin/echo", "hello", "{{ .who }}"},
				WorkingDir: tmp,
				Args: []v1.Param{
					{Name: "who", Type: v1.ParamTypeString, Required: true},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(built.Tools) != 1 {
		t.Fatalf("got %d tools; want 1", len(built.Tools))
	}
	h, ok := built.Helpers["say_hi"]
	if !ok {
		t.Fatal("helper say_hi missing")
	}
	out, err := h(context.Background(), "world")
	if err != nil {
		t.Fatalf("helper: %v", err)
	}
	s, _ := out.(string)
	if !strings.Contains(s, "hello world") {
		t.Errorf("stdout = %q; want hello world", s)
	}
}

func TestBuild_Shell_Rejects(t *testing.T) {
	tests := []struct {
		name    string
		skill   v1.ShellSkill
		wantErr string
	}{
		{
			name: "shell interpreter",
			skill: v1.ShellSkill{
				Command:    []string{"sh", "-c", "echo hi"},
				WorkingDir: t.TempDir(),
			},
			wantErr: "shell interpreter",
		},
		{
			name: "spliced template",
			skill: v1.ShellSkill{
				Command:    []string{"/bin/echo", "-l {{ .path }}"},
				WorkingDir: t.TempDir(),
				Args:       []v1.Param{{Name: "path", Type: v1.ParamTypeString}},
			},
			wantErr: "spliced template",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Build("agent", "", v1.Skills{
				Shell: map[string]v1.ShellSkill{"bad": tt.skill},
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v; want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuild_File_ReadWrite(t *testing.T) {
	tmp := t.TempDir()
	built, err := Build("agent", "", v1.Skills{
		File: &v1.FileSkill{
			WorkingDir: tmp,
			Read:       &v1.FileOp{Enabled: true},
			Write:      &v1.FileOp{Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if _, err := built.Helpers["file_write"](context.Background(), "note.txt", "hi"); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, "note.txt"))
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "hi" {
		t.Errorf("file = %q; want hi", got)
	}

	out, err := built.Helpers["file_read"](context.Background(), "note.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if out.(string) != "hi" {
		t.Errorf("read = %q; want hi", out)
	}
}

func TestBuild_File_Edit(t *testing.T) {
	tmp := t.TempDir()
	built, err := Build("agent", "", v1.Skills{
		File: &v1.FileSkill{
			WorkingDir: tmp,
			Read:       &v1.FileOp{Enabled: true},
			Write:      &v1.FileOp{Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "note.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := built.Helpers["file_edit"](context.Background(), "note.txt", "world", "there"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, "note.txt"))
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "hello there" {
		t.Errorf("file = %q; want %q", got, "hello there")
	}

	if _, err := built.Helpers["file_edit"](context.Background(), "note.txt", "missing", "x"); err == nil {
		t.Fatal("expected error for missing old text")
	}

	if err := os.WriteFile(filepath.Join(tmp, "dup.txt"), []byte("a a"), 0o644); err != nil {
		t.Fatalf("seed dup: %v", err)
	}
	if _, err := built.Helpers["file_edit"](context.Background(), "dup.txt", "a", "b"); err == nil {
		t.Fatal("expected error for non-unique old text")
	}
}

func TestBuild_File_EditRequiresWrite(t *testing.T) {
	built, err := Build("agent", "", v1.Skills{
		File: &v1.FileSkill{
			WorkingDir: t.TempDir(),
			Read:       &v1.FileOp{Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := built.Helpers["file_edit"]; ok {
		t.Fatal("file_edit should not be exposed when Write is disabled")
	}
}

func TestCheckAllow_PortablePaths(t *testing.T) {
	tests := []struct {
		name  string
		allow []string
		path  string
		ok    bool
	}{
		{"empty allow accepts all", nil, "any/file.txt", true},
		{"exact match", []string{"docs/readme.md"}, "docs/readme.md", true},
		{"glob single segment", []string{"docs/*.md"}, "docs/readme.md", true},
		{"glob single segment miss", []string{"docs/*.md"}, "docs/sub/readme.md", false},
		{"doublestar prefix", []string{"src/**"}, "src/a/b/c.go", true},
		{"doublestar tail", []string{"**/*.go"}, "src/a/b/c.go", true},
		{"doublestar miss", []string{"src/**"}, "docs/readme.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAllow(tt.allow, tt.path)
			if (err == nil) != tt.ok {
				t.Errorf("checkAllow(%v, %q) err=%v; ok=%v", tt.allow, tt.path, err, tt.ok)
			}
		})
	}
}

func TestBuild_File_List(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	built, err := Build("agent", "", v1.Skills{
		File: &v1.FileSkill{
			WorkingDir: tmp,
			Read:       &v1.FileOp{Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	out, err := built.Helpers["file_list"](context.Background(), "sub")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got, _ := out.([]string)
	if len(got) != 1 || got[0] != "sub/b.txt" {
		t.Errorf("list = %v; want [sub/b.txt]", got)
	}
}

func TestBuild_File_DenyEscape(t *testing.T) {
	tmp := t.TempDir()
	built, err := Build("agent", "", v1.Skills{
		File: &v1.FileSkill{
			WorkingDir: tmp,
			Read:       &v1.FileOp{Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, err := built.Helpers["file_read"](context.Background(), "../etc/passwd"); err == nil {
		t.Fatal("expected sandbox escape error")
	}
}

func TestBuild_Web_AllowedAndDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("<html><body><h1>ok</h1></body></html>"))
	}))
	defer srv.Close()

	built, err := Build("agent", "", v1.Skills{
		Web: &v1.WebSkill{Enabled: true, URLs: []string{srv.URL}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, err := built.Helpers["web_scrape"](context.Background(), srv.URL); err != nil {
		t.Fatalf("allowed scrape: %v", err)
	}
	if _, err := built.Helpers["web_scrape"](context.Background(), "https://evil.example.com/"); err == nil {
		t.Fatal("expected denied URL error")
	}
}

func TestBuild_Web_RejectEnabledWithEmptyURLs(t *testing.T) {
	_, err := Build("agent", "", v1.Skills{
		Web: &v1.WebSkill{Enabled: true},
	})
	if err == nil || !strings.Contains(err.Error(), "urls is empty") {
		t.Fatalf("err = %v; want urls-empty rejection", err)
	}
}
