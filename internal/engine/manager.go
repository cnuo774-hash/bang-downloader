package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnuo774-hash/bang-downloader/internal/config"
	"github.com/cnuo774-hash/bang-downloader/internal/downloader"
)

type Task struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Total       int64    `json:"total"`
	Completed   int64    `json:"completed"`
	DownloadBPS int64    `json:"downloadBps"`
	Output      string   `json:"output"`
	Error       string   `json:"error,omitempty"`
	UpdatedAt   int64    `json:"updatedAt"`
	AddedAt     int64    `json:"addedAt"`
	FollowedBy  []string `json:"followedBy,omitempty"`
}

type Event struct {
	Kind     string    `json:"kind"`
	Task     Task      `json:"task"`
	Added    bool      `json:"added"`
	Total    int       `json:"total"`
	Revision uint64    `json:"revision"`
	Stats    TaskStats `json:"stats"`
}

type TaskStats struct {
	Active      int   `json:"active"`
	Complete    int   `json:"complete"`
	DownloadBPS int64 `json:"downloadBps"`
}

func (s *TaskStats) adjust(task Task, delta int) {
	if task.Status == "active" {
		s.Active += delta
		s.DownloadBPS += int64(delta) * task.DownloadBPS
	}
	if task.Status == "complete" && len(task.FollowedBy) == 0 {
		s.Complete += delta
	}
}

type TaskPage struct {
	Items    []Task    `json:"items"`
	Total    int       `json:"total"`
	Revision uint64    `json:"revision"`
	Stats    TaskStats `json:"stats"`
}

type Manager struct {
	cmd         *exec.Cmd
	guard       *processGuard
	rpc         *rpcClient
	ctx         context.Context
	cancel      context.CancelFunc
	processDone chan struct{}
	closeOnce   sync.Once
	closeErr    error
	watchDone   chan struct{}
	sessionLock *os.File
	controlMu   sync.Mutex
	historyMu   sync.Mutex

	mu       sync.RWMutex
	tasks    map[string]Task
	sink     func(Event)
	history  string
	revision uint64
	stats    TaskStats
	waiters  map[string]int
}

type rpcTask struct {
	GID             string   `json:"gid"`
	Status          string   `json:"status"`
	TotalLength     string   `json:"totalLength"`
	CompletedLength string   `json:"completedLength"`
	DownloadSpeed   string   `json:"downloadSpeed"`
	Dir             string   `json:"dir"`
	ErrorMessage    string   `json:"errorMessage"`
	FollowedBy      []string `json:"followedBy"`
	Bittorrent      struct {
		Info struct {
			Name string `json:"name"`
		} `json:"info"`
	} `json:"bittorrent"`
	Files []struct {
		Path string `json:"path"`
	} `json:"files"`
}

var taskFields = []string{"gid", "status", "totalLength", "completedLength", "downloadSpeed", "dir", "errorMessage", "bittorrent", "files", "followedBy"}

func New(cfg *config.Config) (*Manager, error) {
	if cfg == nil || cfg.RPCSecret == "" || strings.ContainsAny(cfg.RPCSecret, "\x00\r\n") {
		return nil, errors.New("下载引擎需要非空 RPC 密钥")
	}
	aria2, err := resolveAria2()
	if err != nil {
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	dir := filepath.Join(cache, "Bang")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock, err := lockSession(filepath.Join(dir, "session.lock"))
	if err != nil {
		return nil, err
	}
	started := false
	defer func() {
		if !started {
			_ = lock.Close()
		}
	}()
	// Keep the RPC secret out of the process command line, which other local
	// users can inspect. This private startup file is removed once RPC is ready.
	rpcConfig, err := os.CreateTemp(dir, ".rpc-*.conf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(rpcConfig.Name())
	defer rpcConfig.Close()
	if _, err := fmt.Fprintf(rpcConfig, "rpc-secret=%s\n", cfg.RPCSecret); err != nil {
		return nil, err
	}
	if err := rpcConfig.Close(); err != nil {
		return nil, err
	}
	args := []string{
		"--conf-path=" + rpcConfig.Name(),
		"--enable-rpc=true",
		"--rpc-listen-all=false",
		"--rpc-listen-port=" + strconv.Itoa(port),
		"--dir=" + cfg.Output,
		"--continue=true",
		"--check-integrity=true",
		"--save-session=" + filepath.Join(dir, "session.txt"),
		"--input-file=" + filepath.Join(dir, "session.txt"),
		"--save-session-interval=30",
		"--auto-file-renaming=false",
		"--allow-overwrite=false",
		"--uri-selector=feedback",
		"--max-tries=3",
		"--retry-wait=5",
		"--connect-timeout=15",
		"--timeout=30",
		"--console-log-level=warn",
		"--seed-time=0",
		"--max-download-result=200",
	}
	if cfg.MaxDownload != "" {
		args = append(args, "--max-overall-download-limit="+cfg.MaxDownload)
	}
	if _, err := os.Stat(filepath.Join(dir, "session.txt")); os.IsNotExist(err) {
		if err := os.WriteFile(filepath.Join(dir, "session.txt"), nil, 0o600); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		ctx: ctx, cancel: cancel, processDone: make(chan struct{}), watchDone: make(chan struct{}), sessionLock: lock,
		tasks: make(map[string]Task), history: filepath.Join(dir, "history.json"),
	}
	m.loadHistory()
	cmd := exec.Command(aria2, args...)
	guard, err := configureProcess(cmd)
	if err != nil {
		cancel()
		return nil, err
	}
	m.guard = guard
	writer := log.Writer()
	cmd.Stdout, cmd.Stderr = writer, writer
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("启动 aria2c: %w", err)
	}
	m.cmd = cmd
	if err := guard.attach(cmd); err != nil {
		_ = guard.kill(cmd)
		_ = cmd.Wait()
		_ = guard.close()
		cancel()
		return nil, fmt.Errorf("绑定 aria2c 生命周期: %w", err)
	}
	go func() {
		_ = cmd.Wait()
		close(m.processDone)
	}()
	m.rpc = newRPC(port, cfg.RPCSecret)
	if err := m.waitReady(); err != nil {
		close(m.watchDone)
		_ = m.Close()
		return nil, err
	}
	m.refresh()
	started = true
	go m.watch()
	return m, nil
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("分配 RPC 端口: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func (m *Manager) waitReady() error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-m.processDone:
			return errors.New("aria2c 在 RPC 就绪前退出，请查看 Bang 日志")
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		var version map[string]interface{}
		err := m.rpc.call(ctx, "aria2.getVersion", nil, &version)
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("等待 aria2c RPC 就绪超时")
}

func (m *Manager) Add(ctx context.Context, rawSource, output string) (Task, error) {
	return m.AddWithSources(ctx, []string{rawSource}, output)
}

// AddWithSources adds one download using one or more equivalent HTTP(S) sources.
// aria2 retries and selects another URI when a source times out or fails.
func (m *Manager) AddWithSources(ctx context.Context, rawSources []string, output string) (Task, error) {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	if len(rawSources) == 0 {
		return Task{}, errors.New("下载地址不能为空")
	}
	sources := make([]downloader.Source, 0, len(rawSources))
	for _, raw := range rawSources {
		source, err := downloader.ParseSource(raw)
		if err != nil {
			return Task{}, err
		}
		sources = append(sources, source)
		if len(rawSources) > 1 && (source.Kind != downloader.SourceURI || !(strings.HasPrefix(source.Value, "http:") || strings.HasPrefix(source.Value, "https:"))) {
			return Task{}, errors.New("备用下载源及主来源必须是 HTTP(S) 地址")
		}
	}
	if sources[0].Kind == downloader.SourceTorrent && len(sources) > 1 {
		return Task{}, errors.New("种子文件不能配置备用下载源")
	}
	if len(sources) > 1 && strings.HasPrefix(strings.ToLower(sources[0].Value), "magnet:") {
		return Task{}, errors.New("磁力链接不能配置备用下载源")
	}
	resolvedOutput, err := downloader.ResolveOutput(output)
	if err != nil {
		return Task{}, err
	}
	output = resolvedOutput
	options := map[string]string{"dir": output}
	var gid string
	if sources[0].Kind == downloader.SourceTorrent {
		data, err := os.ReadFile(sources[0].Value)
		if err != nil {
			return Task{}, err
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		err = m.rpc.call(ctx, "aria2.addTorrent", []interface{}{encoded, []string{}, options}, &gid)
	} else {
		uris := make([]string, 0, len(sources))
		for _, source := range sources {
			if source.Kind != downloader.SourceURI {
				return Task{}, errors.New("备用下载源必须是 HTTP(S) 地址")
			}
			uris = append(uris, source.Value)
		}
		err = m.rpc.call(ctx, "aria2.addUri", []interface{}{uris, options}, &gid)
	}
	if err != nil {
		return Task{}, err
	}
	task := Task{ID: gid, Name: displayName(sources[0].Value), Status: "waiting", Output: output, UpdatedAt: time.Now().UnixMilli()}
	m.update(task)
	m.mu.RLock()
	task = m.tasks[gid]
	m.mu.RUnlock()
	return task, nil
}

func displayName(source string) string {
	if strings.HasPrefix(source, "magnet:") {
		return "正在获取磁力元数据"
	}
	name := filepath.Base(source)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "新下载"
	}
	return name
}

func (m *Manager) Wait(gid string) int {
	m.mu.Lock()
	if _, exists := m.tasks[gid]; !exists {
		m.mu.Unlock()
		return 1
	}
	if m.waiters == nil {
		m.waiters = make(map[string]int)
	}
	m.waiters[gid]++
	m.mu.Unlock()
	defer func() { m.mu.Lock(); m.waiters[gid]--; m.mu.Unlock() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return 1
		case <-m.processDone:
			return 1
		case <-ticker.C:
			m.mu.RLock()
			status := m.completionStatus(gid, make(map[string]bool))
			m.mu.RUnlock()
			switch status {
			case "complete":
				return 0
			case "error", "removed":
				return 1
			}
		}
	}
}

// Metadata downloads generate new GIDs. Completion means every descendant has
// finished, rather than only the metadata or remote .torrent transfer.
// The caller holds m.mu for reading.
func (m *Manager) completionStatus(gid string, visited map[string]bool) string {
	if visited[gid] {
		return "error"
	}
	visited[gid] = true
	task, ok := m.tasks[gid]
	if !ok {
		return "waiting"
	}
	if task.Status != "complete" || len(task.FollowedBy) == 0 {
		return task.Status
	}
	status := "complete"
	for _, child := range task.FollowedBy {
		childStatus := m.completionStatus(child, visited)
		if childStatus == "error" || childStatus == "removed" {
			return childStatus
		}
		if childStatus != "complete" {
			status = "waiting"
		}
	}
	return status
}

func (m *Manager) SetEventSink(sink func(Event)) {
	m.mu.Lock()
	m.sink = sink
	m.mu.Unlock()
}

func (m *Manager) List(offset, limit int) []Task {
	return m.Page(offset, limit).Items
}

func (m *Manager) Page(offset, limit int) TaskPage {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		all = append(all, task)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].AddedAt == all[j].AddedAt {
			return all[i].ID > all[j].ID
		}
		return all[i].AddedAt > all[j].AddedAt
	})
	if offset >= len(all) {
		return TaskPage{Items: []Task{}, Total: len(all), Revision: m.revision, Stats: m.stats}
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return TaskPage{Items: all[offset:end], Total: len(all), Revision: m.revision, Stats: m.stats}
}

func (m *Manager) Pause(ctx context.Context, gid string) error {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	var result string
	return m.rpc.call(ctx, "aria2.pause", []interface{}{gid}, &result)
}

func (m *Manager) Resume(ctx context.Context, gid string) error {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	var result string
	return m.rpc.call(ctx, "aria2.unpause", []interface{}{gid}, &result)
}

func (m *Manager) SetMaxDownload(ctx context.Context, limit string) error {
	if limit == "" {
		limit = "0"
	}
	var result string
	return m.rpc.call(ctx, "aria2.changeGlobalOption", []interface{}{map[string]string{
		"max-overall-download-limit": limit,
	}}, &result)
}

func (m *Manager) Remove(ctx context.Context, gid string) error {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	m.mu.RLock()
	task, exists := m.tasks[gid]
	m.mu.RUnlock()
	if !exists {
		return errors.New("下载任务不存在")
	}
	var result string
	// Query the engine because a task may have completed since the last poll.
	var current rpcTask
	err := m.rpc.call(ctx, "aria2.tellStatus", []interface{}{gid, taskFields}, &current)
	if err == nil {
		if !terminal(current.Status) {
			if err := m.rpc.call(ctx, "aria2.forceRemove", []interface{}{gid}, &result); err != nil {
				return err
			}
		}
		if err := m.rpc.call(ctx, "aria2.removeDownloadResult", []interface{}{gid}, &result); err != nil {
			return err
		}
	} else {
		var rpcErr *rpcError
		if !terminal(task.Status) || !errors.As(err, &rpcErr) || rpcErr.Code != 1 {
			return err
		}
	}
	m.deleteTask(gid)
	m.saveHistory()
	return nil
}

func (m *Manager) deleteTask(gid string) {
	m.mu.Lock()
	task, exists := m.tasks[gid]
	if !exists {
		m.mu.Unlock()
		return
	}
	delete(m.tasks, gid)
	m.stats.adjust(task, -1)
	m.revision++
	event := Event{Kind: "remove", Task: task, Total: len(m.tasks), Revision: m.revision, Stats: m.stats}
	sink := m.sink
	m.mu.Unlock()
	if sink != nil {
		sink(event)
	}
}

func (m *Manager) watch() {
	defer close(m.watchDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.processDone:
			return
		case <-ticker.C:
			m.refresh()
		}
	}
}

func (m *Manager) refresh() {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
	defer cancel()
	var groups [][]rpcTask
	methods := []struct {
		name   string
		params []interface{}
	}{
		{"aria2.tellActive", []interface{}{taskFields}},
		{"aria2.tellWaiting", []interface{}{0, 1000, taskFields}},
		{"aria2.tellStopped", []interface{}{0, 1000, taskFields}},
	}
	for _, method := range methods {
		var found []rpcTask
		for offset := 0; ; offset += 1000 {
			params := slices.Clone(method.params)
			if len(params) == 3 {
				params[0] = offset
			}
			var page []rpcTask
			if err := m.rpc.call(ctx, method.name, params, &page); err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Printf("refresh %s: %v", method.name, err)
				}
				return
			}
			found = append(found, page...)
			if len(params) != 3 || len(page) < 1000 {
				break
			}
		}
		groups = append(groups, found)
	}
	for _, group := range groups {
		for _, raw := range group {
			m.update(convertTask(raw))
		}
	}
	m.pruneHistory(ctx)
}

func convertTask(raw rpcTask) Task {
	name := raw.Bittorrent.Info.Name
	if name == "" && len(raw.Files) > 0 {
		name = filepath.Base(raw.Files[0].Path)
	}
	if name == "" {
		name = "下载任务"
	}
	return Task{
		ID: raw.GID, Name: name, Status: raw.Status,
		Total: number(raw.TotalLength), Completed: number(raw.CompletedLength), DownloadBPS: number(raw.DownloadSpeed),
		Output: raw.Dir, Error: raw.ErrorMessage, UpdatedAt: time.Now().UnixMilli(),
		FollowedBy: raw.FollowedBy,
	}
}

func number(value string) int64 {
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func (m *Manager) update(task Task) {
	m.mu.Lock()
	old, exists := m.tasks[task.ID]
	changed := !exists || old.Status != task.Status || old.Completed != task.Completed || old.Total != task.Total || old.DownloadBPS != task.DownloadBPS || old.Name != task.Name || old.Error != task.Error || old.Output != task.Output || !slices.Equal(old.FollowedBy, task.FollowedBy)
	if !changed {
		m.mu.Unlock()
		return
	}
	if exists {
		task.AddedAt = old.AddedAt
	} else if task.AddedAt == 0 {
		task.AddedAt = time.Now().UnixMilli()
	}
	m.tasks[task.ID] = task
	if exists {
		m.stats.adjust(old, -1)
	}
	m.stats.adjust(task, 1)
	m.revision++
	event := Event{Kind: "upsert", Task: task, Added: !exists, Total: len(m.tasks), Revision: m.revision, Stats: m.stats}
	sink := m.sink
	m.mu.Unlock()
	if sink != nil {
		sink(event)
	}
	if task.Status == "complete" || task.Status == "error" || task.Status == "removed" {
		m.saveHistory()
	}
}

func (m *Manager) loadHistory() {
	data, err := os.ReadFile(m.history)
	if err != nil {
		return
	}
	var tasks []Task
	if json.Unmarshal(data, &tasks) != nil {
		return
	}
	for _, task := range tasks {
		if !terminal(task.Status) {
			continue
		}
		if task.AddedAt == 0 {
			task.AddedAt = task.UpdatedAt
		}
		m.tasks[task.ID] = task
		m.stats.adjust(task, 1)
	}
}

func (m *Manager) saveHistory() {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	tasks := m.terminalTasks()
	if len(tasks) > 200 {
		tasks = tasks[:200]
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.history), ".history-*")
	if err != nil {
		return
	}
	name := tmp.Name()
	defer os.Remove(name)
	_ = tmp.Chmod(0o600)
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr == nil && closeErr == nil {
		_ = os.Rename(name, m.history)
	}
}

func terminal(status string) bool {
	return status == "complete" || status == "error" || status == "removed"
}

func (m *Manager) terminalTasks() []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]Task, 0)
	for _, task := range m.tasks {
		if terminal(m.completionStatus(task.ID, make(map[string]bool))) {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].AddedAt == tasks[j].AddedAt {
			return tasks[i].ID > tasks[j].ID
		}
		return tasks[i].AddedAt > tasks[j].AddedAt
	})
	return tasks
}

func (m *Manager) pruneHistory(ctx context.Context) {
	tasks := m.terminalTasks()
	if len(tasks) <= 200 {
		return
	}
	m.mu.RLock()
	protected := make(map[string]bool)
	var protect func(string)
	protect = func(id string) {
		if protected[id] {
			return
		}
		protected[id] = true
		for _, child := range m.tasks[id].FollowedBy {
			protect(child)
		}
	}
	for id, count := range m.waiters {
		if count > 0 {
			protect(id)
		}
	}
	m.mu.RUnlock()
	for _, task := range tasks[200:] {
		if protected[task.ID] {
			continue
		}
		var result string
		_ = m.rpc.call(ctx, "aria2.removeDownloadResult", []interface{}{task.ID}, &result)
		m.deleteTask(task.ID)
	}
	m.saveHistory()
}

func (m *Manager) Close() error {
	m.closeOnce.Do(func() {
		m.cancel()
		<-m.watchDone
		m.controlMu.Lock()
		defer m.controlMu.Unlock()
		m.saveHistory()
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		var saved string
		if err := m.rpc.call(saveCtx, "aria2.saveSession", nil, &saved); err != nil {
			select {
			case <-m.processDone:
			default:
				log.Printf("save aria2 session: %v", err)
			}
		}
		saveCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		var result string
		_ = m.rpc.call(ctx, "aria2.shutdown", nil, &result)
		select {
		case <-m.processDone:
		case <-time.After(2 * time.Second):
			m.closeErr = m.guard.kill(m.cmd)
			select {
			case <-m.processDone:
			case <-time.After(2 * time.Second):
				if m.closeErr == nil {
					m.closeErr = errors.New("aria2c 进程未能在超时内退出")
				}
			}
		}
		if err := m.guard.close(); m.closeErr == nil {
			m.closeErr = err
		}
		if err := m.sessionLock.Close(); m.closeErr == nil {
			m.closeErr = err
		}
	})
	return m.closeErr
}

func OpenPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer.exe", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
