package main

import "testing"

func TestParseCLIArgsAcceptsOutputAfterSource(t *testing.T) {
	opts, err := parseCLIArgs([]string{"magnet:?xt=urn:btih:0123456789abcdef", "-o", "./downloads"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.source == "" || opts.output != "./downloads" {
		t.Fatalf("unexpected options: %#v", opts)
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
