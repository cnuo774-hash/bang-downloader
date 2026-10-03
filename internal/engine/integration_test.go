//go:build integration

package engine

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cnuo774-hash/bang-downloader/internal/config"
)

func isolatedConfig(t *testing.T) *config.Config {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	t.Setenv("LOCALAPPDATA", filepath.Join(base, "cache"))
	if os.Getenv("BANG_REQUIRE_EMBEDDED") == "1" {
		name := fmt.Sprintf("binaries/%s-%s-aria2c", runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		data, err := aria2Assets.ReadFile(name)
		if err != nil || bytes.HasPrefix(data, []byte("BANG_ARIA2_PLACEHOLDER")) {
			t.Fatalf("release requires embedded aria2: %s: %v", name, err)
		}
	}
	return &config.Config{RPCSecret: strings.Repeat("a", 64), Output: base}
}

func startTestEngine(t *testing.T, cfg *config.Config) *Manager {
	t.Helper()
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	var info struct {
		Version string `json:"version"`
	}
	if err := m.rpc.call(context.Background(), "aria2.getVersion", nil, &info); err != nil || info.Version != aria2Version {
		t.Fatalf("unexpected engine version: %+v, %v", info, err)
	}
	return m
}

func awaitStatus(t *testing.T, m *Manager, gid, status string) Task {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		m.refresh()
		m.mu.RLock()
		task := m.tasks[gid]
		m.mu.RUnlock()
		if task.Status == status {
			return task
		}
		if task.Status == "error" && status != "error" {
			t.Fatalf("task failed: %+v", task)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach %s", gid, status)
	return Task{}
}

func TestIntegrationHTTPDownloadMirrorsHistoryAndRemoval(t *testing.T) {
	cfg := isolatedConfig(t)
	payload := bytes.Repeat([]byte("bang integration download\n"), 4096)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/missing/") {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "payload.bin", time.Time{}, bytes.NewReader(payload))
	}))
	defer server.Close()
	m := startTestEngine(t, cfg)
	task, err := m.AddWithSources(context.Background(), []string{server.URL + "/missing/payload.bin", server.URL + "/mirror/payload.bin"}, cfg.Output)
	if err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, m, task.ID, "complete")
	if m.Wait(task.ID) != 0 {
		t.Fatal("completed download reported failure")
	}
	data, err := os.ReadFile(filepath.Join(cfg.Output, "payload.bin"))
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("download content mismatch: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m = startTestEngine(t, cfg)
	if m.Page(0, 100).Total != 1 {
		t.Fatal("completed history not restored")
	}
	if err := m.Remove(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	m.refresh()
	if m.Page(0, 100).Total != 0 {
		t.Fatal("deleted task reappeared")
	}
	if _, err := os.Stat(filepath.Join(cfg.Output, "payload.bin")); err != nil {
		t.Fatal("removing task deleted downloaded file")
	}
}

func TestIntegrationPauseRestartResumeAndSessionLock(t *testing.T) {
	cfg := isolatedConfig(t)
	cfg.MaxDownload = "64K"
	payload := bytes.Repeat([]byte("0123456789abcdef"), 65536)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "resume.bin", time.Time{}, bytes.NewReader(payload))
	}))
	defer server.Close()
	m := startTestEngine(t, cfg)
	if duplicate, err := New(cfg); err == nil {
		duplicate.Close()
		t.Fatal("concurrent session accepted")
	}
	task, err := m.Add(context.Background(), server.URL+"/resume.bin", cfg.Output)
	if err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, m, task.ID, "active")
	if err := m.Pause(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, m, task.ID, "paused")
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m = startTestEngine(t, cfg)
	page := m.Page(0, 100)
	if page.Total != 1 {
		data, _ := os.ReadFile(filepath.Join(filepath.Dir(m.history), "session.txt"))
		t.Fatalf("session lost or duplicated: %+v, session: %s", page, data)
	}
	restored := page.Items[0]
	if restored.Status == "paused" {
		if err := m.Resume(context.Background(), restored.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.SetMaxDownload(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, m, restored.ID, "complete")
	data, err := os.ReadFile(filepath.Join(cfg.Output, "resume.bin"))
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("resumed file corrupted: %v", err)
	}
}

func TestIntegrationRemoteTorrentWaitsForPayload(t *testing.T) {
	cfg := isolatedConfig(t)
	payload := bytes.Repeat([]byte("torrent-webseed-test"), 1024)
	var torrent []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test.torrent" {
			w.Header().Set("Content-Type", "application/x-bittorrent")
			_, _ = w.Write(torrent)
			return
		}
		http.ServeContent(w, r, "torrent.bin", time.Time{}, bytes.NewReader(payload))
	}))
	defer server.Close()
	// A self-contained torrent with a local web seed, never contacting real peers.
	piece := sha1.Sum(payload)
	url := server.URL + "/torrent.bin"
	torrent = []byte(fmt.Sprintf("d4:infod6:lengthi%de4:name11:torrent.bin12:piece lengthi32768e6:pieces20:", len(payload)))
	torrent = append(torrent, piece[:]...)
	torrent = append(torrent, []byte(fmt.Sprintf("7:privatei1ee8:url-list%d:%se", len(url), url))...)
	m := startTestEngine(t, cfg)
	var result string
	if err := m.rpc.call(context.Background(), "aria2.changeGlobalOption", []interface{}{map[string]string{"max-overall-download-limit": "16K"}}, &result); err != nil {
		t.Fatal(err)
	}
	task, err := m.Add(context.Background(), server.URL+"/test.torrent", cfg.Output)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { done <- m.Wait(task.ID) }()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatal("torrent download failed")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("torrent payload did not finish")
	}
	data, err := os.ReadFile(filepath.Join(cfg.Output, "torrent.bin"))
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("metadata completion mistaken for payload: %v", err)
	}
	m.mu.RLock()
	followed := len(m.tasks[task.ID].FollowedBy)
	m.mu.RUnlock()
	if followed == 0 {
		t.Fatal("test did not exercise generated torrent task")
	}
}

func TestIntegrationFailedDownloadAndEngineExit(t *testing.T) {
	cfg := isolatedConfig(t)
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	m := startTestEngine(t, cfg)
	task, err := m.Add(context.Background(), server.URL+"/missing.bin", cfg.Output)
	if err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, m, task.ID, "error")
	if m.Wait(task.ID) != 1 {
		t.Fatal("failed download reported success")
	}
	if err := m.Remove(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { done <- m.Wait("unknown") }()
	select {
	case code := <-done:
		if code != 1 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait hung after engine exit")
	}
	// No sensitive RPC secret should be serialized to task history.
	m.saveHistory()
	data, err := os.ReadFile(m.history)
	if err != nil || !json.Valid(data) || bytes.Contains(data, []byte(cfg.RPCSecret)) {
		t.Fatal("invalid or sensitive history")
	}
}
