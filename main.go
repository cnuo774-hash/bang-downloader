package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cnuo774-hash/bang-downloader/internal/config"
	"github.com/cnuo774-hash/bang-downloader/internal/downloader"
	"github.com/cnuo774-hash/bang-downloader/internal/engine"
	"github.com/cnuo774-hash/bang-downloader/internal/logging"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var frontend embed.FS

const version = "2.0.0"

type cliOptions struct {
	output  string
	source  string
	ui      bool
	version bool
	help    bool
}

func main() {
	if len(os.Args) == 1 {
		if err := runUI(); err != nil {
			fmt.Fprintln(os.Stderr, "Bang:", err)
			os.Exit(1)
		}
		return
	}
	os.Exit(runCLI(os.Args[1:]))
}

func openConfig() (*config.Config, error) {
	path, err := config.Path()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	if cfg.Dirty() {
		if err := config.Save(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func runCLI(args []string) int {
	opts, err := parseCLIArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "参数错误：", err)
		printUsage(os.Stderr)
		return 2
	}
	if opts.help {
		printUsage(os.Stdout)
		return 0
	}
	if opts.version {
		fmt.Printf("bang %s (aria2 %s)\n", version, engine.Aria2Version())
		return 0
	}
	if opts.ui {
		if opts.source != "" || opts.output != "" {
			fmt.Fprintln(os.Stderr, "--ui 不能与下载来源或 --output 同时使用")
			return 2
		}
		if err := runUI(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if opts.source == "" {
		printUsage(os.Stderr)
		return 2
	}
	cfg, err := openConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败：", err)
		return 1
	}
	manager, err := engine.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "下载引擎启动失败：", err)
		return 1
	}
	defer manager.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = manager.Close()
	}()
	if opts.output == "" {
		opts.output = cfg.Output
	}
	target, err := downloader.ResolveOutput(opts.output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "保存路径无效：", err)
		return 2
	}
	task, err := manager.Add(ctx, opts.source, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "添加任务失败：", err)
		return 1
	}
	fmt.Printf("任务 %s 已添加，保存到 %s\n", task.ID, target)
	return manager.Wait(task.ID)
}

func parseCLIArgs(args []string) (cliOptions, error) {
	var opts cliOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-h" || arg == "--help":
			opts.help = true
		case arg == "-v" || arg == "--version":
			opts.version = true
		case arg == "--ui":
			opts.ui = true
		case arg == "-o" || arg == "--output":
			index++
			if index >= len(args) {
				return cliOptions{}, fmt.Errorf("%s 缺少目录参数", arg)
			}
			opts.output = args[index]
		case strings.HasPrefix(arg, "--output="):
			opts.output = strings.TrimPrefix(arg, "--output=")
			if opts.output == "" {
				return cliOptions{}, fmt.Errorf("--output 缺少目录参数")
			}
		case strings.HasPrefix(arg, "-"):
			return cliOptions{}, fmt.Errorf("未知参数 %s", arg)
		default:
			if opts.source != "" {
				return cliOptions{}, fmt.Errorf("只能指定一个下载来源")
			}
			opts.source = arg
		}
	}
	return opts, nil
}

func printUsage(output *os.File) {
	fmt.Fprintln(output, "用法：")
	fmt.Fprintln(output, "  bang <磁力链接|种子文件|HTTP 地址> [-o 保存目录]")
	fmt.Fprintln(output, "  bang --ui")
	fmt.Fprintln(output, "  bang --version")
}

func runUI() error {
	if bindingMode {
		return wails.Run(&options.App{
			AssetServer: &assetserver.Options{Assets: frontend},
			Bind:        []interface{}{NewApp(nil, &config.Config{})},
		})
	}
	logging.Setup()
	cfg, err := openConfig()
	if err != nil {
		return err
	}
	manager, err := engine.New(cfg)
	if err != nil {
		return err
	}
	defer manager.Close()
	app := NewApp(manager, cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = manager.Close()
	}()
	return wails.Run(&options.App{
		Title: "Bang Downloader",
		Width: 1180, Height: 760, MinWidth: 900, MinHeight: 620,
		AssetServer:      &assetserver.Options{Assets: frontend},
		BackgroundColour: &options.RGBA{R: 247, G: 250, B: 249, A: 1},
		Mac:              &mac.Options{TitleBar: mac.TitleBarHiddenInset(), About: &mac.AboutInfo{Title: "Bang Downloader", Message: "本地磁力与种子下载器"}},
		OnStartup:        app.startup,
		OnShutdown:       func(ctx context.Context) { _ = manager.Close() },
		Bind:             []interface{}{app},
	})
}
