package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/cnuo774-hash/bang-downloader/internal/config"
	"github.com/cnuo774-hash/bang-downloader/internal/downloader"
	"github.com/cnuo774-hash/bang-downloader/internal/engine"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var downloadLimitPattern = regexp.MustCompile(`(?i)^(0|[1-9][0-9]*[KMG]?)$`)

type App struct {
	manager *engine.Manager
	cfg     *config.Config
	ctx     context.Context
	mu      sync.RWMutex
}

func NewApp(manager *engine.Manager, cfg *config.Config) *App {
	return &App{manager: manager, cfg: cfg}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.SetEventSink(func(event engine.Event) {
		runtime.EventsEmit(a.ctx, "task:update", event)
	})
}

func (a *App) Add(source, output string) (engine.Task, error) {
	target, err := downloader.ResolveOutput(output)
	if err != nil {
		return engine.Task{}, err
	}
	return a.manager.Add(a.ctx, source, target)
}

func (a *App) ListTasks(offset, limit int) engine.TaskPage {
	return a.manager.Page(offset, limit)
}

func (a *App) Config() config.PublicConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Public()
}

func (a *App) SetOutput(output string) error {
	target, err := downloader.ResolveOutput(output)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Output = target
	return config.Save(a.cfg)
}

func (a *App) SaveSettings(output, maxDownload string) error {
	target, err := downloader.ResolveOutput(output)
	if err != nil {
		return err
	}
	limit := strings.ToUpper(strings.TrimSpace(maxDownload))
	if limit != "" && !downloadLimitPattern.MatchString(limit) {
		return fmt.Errorf("限速格式无效，请使用 512K、10M、1G 或留空")
	}
	if err := a.manager.SetMaxDownload(a.ctx, limit); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Output = target
	a.cfg.MaxDownload = limit
	return config.Save(a.cfg)
}

func (a *App) OpenOutput() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return engine.OpenPath(a.cfg.Output)
}

func (a *App) Pause(id string) error  { return a.manager.Pause(a.ctx, id) }
func (a *App) Resume(id string) error { return a.manager.Resume(a.ctx, id) }
func (a *App) Remove(id string) error { return a.manager.Remove(a.ctx, id) }

func (a *App) ChooseTorrent() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "选择种子文件",
		Filters: []runtime.FileFilter{{DisplayName: "BitTorrent 种子 (*.torrent)", Pattern: "*.torrent"}},
	})
}

func (a *App) ChooseOutput() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择保存目录"})
}
