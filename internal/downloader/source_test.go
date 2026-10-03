package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMagnet(t *testing.T) {
	got, err := ParseSource("magnet:?xt=urn:btih:0123456789abcdef")
	if err != nil || got.Kind != SourceURI {
		t.Fatalf("ParseSource() = %#v, %v", got, err)
	}
}

func TestHTTPSourceValidation(t *testing.T) {
	for _, raw := range []string{"https://example.com/file.zip", "http://127.0.0.1:8080/file"} {
		if source, err := ParseSource(raw); err != nil || source.Kind != SourceURI {
			t.Fatalf("valid source %q rejected: %v", raw, err)
		}
	}
	for _, raw := range []string{"http:///file.zip", "https:relative", "https://", "https://example.com/a\nb", "magnet:?dn=missing-hash"} {
		if _, err := ParseSource(raw); err == nil {
			t.Fatalf("accepted invalid source %q", raw)
		}
	}
}

func TestOutputRejectsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveOutput(path); err == nil {
		t.Fatal("accepted file as directory")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "keep" {
		t.Fatal("existing file was modified")
	}
}

func TestOutputRejectsSessionLineBreaks(t *testing.T) {
	if _, err := ResolveOutput(filepath.Join(t.TempDir(), "folder\nwith-newline")); err == nil {
		t.Fatal("accepted a directory that cannot safely round-trip through an aria2 session")
	}
}

func TestResolveRelativeOutput(t *testing.T) {
	base := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveOutput("downloads")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "downloads")
	want, _ = filepath.EvalSymlinks(want)
	got, _ = filepath.EvalSymlinks(got)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
