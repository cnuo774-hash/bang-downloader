package main

import (
	"encoding/json"
	"github.com/cnuo774-hash/bang-downloader/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCLIArgsAcceptsOutputAfterSource(t *testing.T) {
	opts, err := parseCLIArgs([]string{"magnet:?xt=urn:btih:0123456789abcdef", "-o", "./downloads"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.source == "" || opts.output != "./downloads" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestCLIRejectsMissingOutput(t *testing.T) {
	for _, args := range [][]string{{"-o"}, {"--output", ""}, {"-o", "--ui"}, {"--output="}} {
		if _, err := parseCLIArgs(args); err == nil {
			t.Fatalf("accepted missing output: %v", args)
		}
	}
}

func TestSetOutputFailurePreservesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	before := cfg.Output
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	app := NewApp(nil, cfg)
	if err := app.SetOutput(t.TempDir()); err == nil {
		t.Fatal("expected persistence error")
	}
	if cfg.Output != before {
		t.Fatal("failed settings save changed effective configuration")
	}
}

func TestReleaseVersionsMatch(t *testing.T) {
	for _, file := range []string{"frontend/package.json", "wails.json"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			Version string `json:"version"`
			Info    struct {
				ProductVersion string `json:"productVersion"`
			} `json:"info"`
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		got := document.Version
		if got == "" {
			got = document.Info.ProductVersion
		}
		if got != version {
			t.Fatalf("%s version %s differs from %s", file, got, version)
		}
	}
}

func TestParseCLIArgsAcceptsOutputBeforeSource(t *testing.T) {
	opts, err := parseCLIArgs([]string{"--output=/tmp/downloads", "file.torrent"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.source != "file.torrent" || opts.output != "/tmp/downloads" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseCLIArgsRejectsMultipleSources(t *testing.T) {
	if _, err := parseCLIArgs([]string{"one.torrent", "two.torrent"}); err == nil {
		t.Fatal("expected multiple sources to fail")
	}
}

func TestParseCLIArgsAcceptsMirrors(t *testing.T) {
	opts, err := parseCLIArgs([]string{
		"https://primary.example/file.zip",
		"--mirror", "https://mirror-a.example/file.zip",
		"--mirror=https://mirror-b.example/file.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.mirrors) != 2 {
		t.Fatalf("expected two mirrors, got %#v", opts.mirrors)
	}
}
