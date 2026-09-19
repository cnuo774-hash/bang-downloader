package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAria2IsStable(t *testing.T) {
	data := []byte("test aria2 executable")
	first, err := extractAria2(data, "aria2c-test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := extractAria2(data, "aria2c-test")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("cache path changed: %q != %q", first, second)
	}
	got, err := os.ReadFile(first)
	if err != nil || string(got) != string(data) {
		t.Fatalf("extracted data = %q, %v", got, err)
	}
	if filepath.Dir(first) == "." {
		t.Fatal("aria2 must not be extracted into the current directory")
	}
}
