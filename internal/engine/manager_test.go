package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{ctx: context.Background(), tasks: make(map[string]Task), history: filepath.Join(t.TempDir(), "history.json")}
}

func TestCompletionFollowsMetadataChildren(t *testing.T) {
	m := testManager(t)
	m.tasks["metadata"] = Task{ID: "metadata", Status: "complete", FollowedBy: []string{"file"}}
	if got := m.completionStatus("metadata", map[string]bool{}); got != "waiting" {
		t.Fatal(got)
	}
	m.tasks["file"] = Task{ID: "file", Status: "active"}
	if got := m.completionStatus("metadata", map[string]bool{}); got != "waiting" {
		t.Fatal(got)
	}
	m.tasks["file"] = Task{ID: "file", Status: "complete"}
	if got := m.completionStatus("metadata", map[string]bool{}); got != "complete" {
		t.Fatal(got)
	}
	m.tasks["file"] = Task{ID: "file", Status: "error"}
	if got := m.completionStatus("metadata", map[string]bool{}); got != "error" {
		t.Fatal(got)
	}
}

func TestWaitReturnsWhenEngineExits(t *testing.T) {
	m := testManager(t)
	m.tasks["missing"] = Task{ID: "missing", Status: "active"}
	m.processDone = make(chan struct{})
	close(m.processDone)
	if got := m.Wait("missing"); got != 1 {
		t.Fatal(got)
	}
}

func TestRetentionKeepsWaitedMetadataAndPayload(t *testing.T) {
	m := testManager(t)
	for i := 0; i < 210; i++ {
		m.tasks[fmt.Sprint(i)] = Task{ID: fmt.Sprint(i), Status: "complete", AddedAt: int64(i + 10)}
	}
	m.tasks["metadata"] = Task{ID: "metadata", Status: "complete", AddedAt: 1, FollowedBy: []string{"payload"}}
	m.tasks["payload"] = Task{ID: "payload", Status: "complete", AddedAt: 2}
	m.waiters = map[string]int{"metadata": 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"result":"OK"}`) }))
	defer server.Close()
	m.rpc = newRPC(0, "secret")
	m.rpc.url = server.URL
	m.pruneHistory(context.Background())
	if m.completionStatus("metadata", map[string]bool{}) != "complete" {
		t.Fatal("retention discarded a CLI completion dependency")
	}
}

func TestPageOrderDoesNotChangeOnProgress(t *testing.T) {
	m := testManager(t)
	for i := 0; i < 250; i++ {
		m.update(Task{ID: fmt.Sprintf("%03d", i), Status: "active", AddedAt: int64(i + 1)})
	}
	before := m.Page(0, 100)
	m.update(Task{ID: "000", Status: "active", Completed: 100})
	after := m.Page(0, 100)
	if !reflect.DeepEqual(before.Items, after.Items) {
		t.Fatal("progress reordered pages")
	}
	if after.Total != 250 || after.Revision <= before.Revision {
		t.Fatalf("invalid page: %+v", after)
	}
	if len(m.Page(200, 100).Items) != 50 {
		t.Fatal("last page missing")
	}
}

func TestHistoryOnlyRestoresTerminalTasks(t *testing.T) {
	m := testManager(t)
	m.update(Task{ID: "active", Status: "active"})
	m.update(Task{ID: "done", Status: "complete"})
	m.saveHistory()
	data, err := os.ReadFile(m.history)
	if err != nil {
		t.Fatal(err)
	}
	var history []Task
	if err := json.Unmarshal(data, &history); err != nil || len(history) != 1 || history[0].ID != "done" {
		t.Fatalf("%s: %v", data, err)
	}
	m.tasks = map[string]Task{}
	m.loadHistory()
	if len(m.tasks) != 1 {
		t.Fatal("stale live tasks restored")
	}
}

func TestRemoveUsesTerminalRPCAndEmitsDeletion(t *testing.T) {
	m := testManager(t)
	m.update(Task{ID: "done", Status: "active"}) // The engine has already completed it.
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		methods = append(methods, request.Method)
		if request.Method == "aria2.tellStatus" {
			fmt.Fprint(w, `{"result":{"gid":"done","status":"complete"}}`)
		} else {
			fmt.Fprint(w, `{"result":"OK"}`)
		}
	}))
	defer server.Close()
	m.rpc = newRPC(0, "secret")
	m.rpc.url = server.URL
	var event Event
	m.SetEventSink(func(e Event) { event = e })
	if err := m.Remove(context.Background(), "done"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(methods, []string{"aria2.tellStatus", "aria2.removeDownloadResult"}) {
		t.Fatal(methods)
	}
	if event.Kind != "remove" || event.Total != 0 || m.Page(0, 100).Total != 0 {
		t.Fatalf("deletion not reflected: %+v", event)
	}
}

func TestMirrorsMustAllBeHTTP(t *testing.T) {
	m := testManager(t)
	_, err := m.AddWithSources(context.Background(), []string{"https://example.com/file", "magnet:?xt=urn:btih:test"}, t.TempDir())
	if err == nil {
		t.Fatal("accepted magnet as HTTP mirror")
	}
}

func TestSessionLockPreventsConcurrentInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.lock")
	first, err := lockSession(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, err := lockSession(path); err == nil {
		second.Close()
		t.Fatal("accepted concurrent session writer")
	}
	first.Close()
	third, err := lockSession(path)
	if err != nil {
		t.Fatal(err)
	}
	third.Close()
}

func TestRefreshPaginatesMoreThan1000WaitingTasks(t *testing.T) {
	m := testManager(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		tasks := []rpcTask{}
		if req.Method == "aria2.tellWaiting" {
			offset := int(req.Params[1].(float64))
			for i := offset; i < offset+1000 && i < 1005; i++ {
				tasks = append(tasks, rpcTask{GID: fmt.Sprint(i), Status: "waiting"})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": tasks})
	}))
	defer server.Close()
	m.rpc = newRPC(0, "secret")
	m.rpc.url = server.URL
	m.refresh()
	if m.Page(0, 100).Total != 1005 {
		t.Fatal("waiting tasks truncated")
	}
}

func TestHistoryRetentionPreservesLiveTasks(t *testing.T) {
	m := testManager(t)
	for i := 0; i < 210; i++ {
		m.tasks[fmt.Sprint(i)] = Task{ID: fmt.Sprint(i), Status: "complete", AddedAt: int64(i)}
	}
	m.tasks["live"] = Task{ID: "live", Status: "active"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"result":"OK"}`) }))
	defer server.Close()
	m.rpc = newRPC(0, "secret")
	m.rpc.url = server.URL
	m.pruneHistory(context.Background())
	if m.Page(0, 100).Total != 201 || m.tasks["live"].Status != "active" {
		t.Fatal("history retention removed a live task")
	}
}

func TestRemoveHistoricalTaskMissingFromEngine(t *testing.T) {
	m := testManager(t)
	m.update(Task{ID: "old", Status: "complete", UpdatedAt: time.Now().UnixMilli()})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":1,"message":"GID not found"}}`)
	}))
	defer server.Close()
	m.rpc = newRPC(0, "secret")
	m.rpc.url = server.URL
	if err := m.Remove(context.Background(), "old"); err != nil {
		t.Fatal(err)
	}
	if len(m.tasks) != 0 {
		t.Fatal("historical task not deleted")
	}
}

func TestSummaryIncludesUnloadedTasks(t *testing.T) {
	m := testManager(t)
	for i := 0; i < 250; i++ {
		m.update(Task{ID: fmt.Sprint(i), Status: "active", DownloadBPS: 10})
	}
	page := m.Page(0, 100)
	if len(page.Items) != 100 || page.Stats.Active != 250 || page.Stats.DownloadBPS != 2500 {
		t.Fatalf("stats excluded unloaded tasks: %+v", page.Stats)
	}
	m.update(Task{ID: "0", Status: "complete"})
	if got := m.Page(0, 100).Stats; got.Active != 249 || got.Complete != 1 || got.DownloadBPS != 2490 {
		t.Fatal(got)
	}
	m.deleteTask("0")
	if m.Page(0, 100).Stats.Complete != 0 {
		t.Fatal("deleted task still counted")
	}
}
