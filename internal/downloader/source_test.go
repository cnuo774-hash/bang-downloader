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
