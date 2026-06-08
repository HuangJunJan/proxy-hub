// Command proxy-hub 是 proxy-hub 的瘦入口：
// 载配置 → 开 DB → 起 Gin HTTP 服务 → 优雅停机。预留 CLI 子命令骨架。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/huangjunjan/proxy-hub/internal/adaptor/claude" // 注册 anthropic 适配器
	_ "github.com/huangjunjan/proxy-hub/internal/adaptor/openai" // 注册 openai 适配器
	"github.com/huangjunjan/proxy-hub/internal/api"
	"github.com/huangjunjan/proxy-hub/internal/apikey"
	"github.com/huangjunjan/proxy-hub/internal/buildinfo"
	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/config"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
	"github.com/huangjunjan/proxy-hub/internal/health"
	"github.com/huangjunjan/proxy-hub/internal/mcp"
	"github.com/huangjunjan/proxy-hub/internal/relay"
	"github.com/huangjunjan/proxy-hub/internal/selector"
	"github.com/huangjunjan/proxy-hub/internal/stats"
	"github.com/huangjunjan/proxy-hub/internal/store"
	"github.com/huangjunjan/proxy-hub/internal/usage"
)

// shutdownTimeout 是优雅停机时等待在途请求完成的上限。
const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		// 启动期错误直接打到 stderr 并以非零码退出。
		fmt.Fprintf(os.Stderr, "proxy-hub 启动失败: %v\n", err)
		os.Exit(1)
	}
}

// run 承载主流程，返回 error 便于在 main 中统一处理退出码。
func run() error {
	// CLI 子命令骨架：第一个非 flag 参数若为已知子命令则分派（M4 实现 mcp）。
	// 例如 `proxy-hub mcp sync`。M1 仅占位。
	if len(os.Args) > 1 && !isFlag(os.Args[1]) {
		return dispatchSubcommand(os.Args[1], os.Args[2:])
	}

	configPath := flag.String("config", "config.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(buildinfo.Version)
		return nil
	}

	// 1. 加载配置。
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 配置日志（slog）。须在 EnsureAdminKey 打印前设置好。
	setupLogger(cfg)

	// 2. 确保 admin_key（为空则生成并打印一次）。
	if err := cfg.EnsureAdminKey(); err != nil {
		return err
	}
	if cfg.AdminKeyGenerated() {
		// 仅首次生成打印一次；密钥仅此一刻可见，运维须妥善保存。
		slog.Warn("已自动生成 admin_key（请妥善保存，仅打印此一次）", "admin_key", cfg.AdminKey)
	}

	slog.Info("proxy-hub 启动中",
		"version", buildinfo.Version,
		"addr", cfg.Server.Addr,
		"data_dir", cfg.DataDir,
	)

	// 3. 打开 DB（含迁移）。
	st, err := store.Open(cfg)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer func() { _ = st.Close() }()

	// 3b. M2 装配：凭证库 + dao + 路由索引 + 编排器 + 鉴权缓存 + 健康镜像 + 中转引擎。
	creds, err := credstore.Open(cfg.AuthsDir())
	if err != nil {
		return fmt.Errorf("打开凭证库失败: %w", err)
	}
	defer func() { _ = creds.Close() }()

	startupCtx := context.Background()
	dao := channel.NewDAO(st)
	routeIndex := channel.NewRouteIndex()
	manager := channel.NewManager(dao, creds, routeIndex)
	if err := manager.LoadRouteIndex(startupCtx); err != nil {
		return fmt.Errorf("装配路由索引失败: %w", err)
	}

	keyCache := apikey.NewCache(manager.LookupKeyByHash)

	healthStore := relay.NewHealthStore(dao)
	healthStates, err := healthStore.Load(startupCtx)
	if err != nil {
		return fmt.Errorf("装配渠道健康镜像失败: %w", err)
	}
	healthMirror := relay.NewHealthMirror(healthStore.Persist)
	healthMirror.Load(healthStates)

	// 3c. M3 装配：统计 DAO + 定价表（内置种子 + DB）+ 用量发射器（可选同步兜底）+ 采集器。
	statsDAO := stats.NewDAO(st)
	if entries, serr := stats.SeedEntries(); serr != nil {
		slog.Warn("加载内置定价种子失败，跳过种入", "error", serr)
	} else if err := statsDAO.SeedPricing(startupCtx, entries, time.Now().UTC().Format(time.RFC3339)); err != nil {
		slog.Warn("种入定价失败，继续（管理端可手工配置）", "error", err)
	}
	pricingTable := stats.NewTable()
	priceRows, err := statsDAO.PriceRows(startupCtx)
	if err != nil {
		return fmt.Errorf("装配定价表失败: %w", err)
	}
	pricingTable.Load(priceRows)

	// 用量发射器：sync_fallback_on_full=true 时通道满改同步插入兜底（绝不丢计费，代价是热路径偶发阻塞）。
	var onFull func(usage.Event)
	if cfg.Stats.SyncFallbackOnFull {
		onFull = func(ev usage.Event) {
			if err := statsDAO.InsertLogsBatch(context.Background(), []usage.Event{ev}); err != nil {
				slog.Error("用量同步兜底插入失败", "error", err)
			}
		}
	}
	emitter := usage.NewEmitter(cfg.Relay.UsageBuffer, onFull)

	collector := stats.NewCollector(stats.CollectorConfig{
		DAO:           statsDAO,
		Emitter:       emitter,
		BatchSize:     cfg.Stats.BatchSize,
		BatchInterval: cfg.Stats.BatchInterval(),
		FlushInterval: cfg.Stats.FlushInterval(),
	})
	usageDone := collector.Run(context.Background()) // 停机经 emitter.Close() 排空触发最终 flush

	engine := relay.NewEngine(relay.Config{
		Index:              routeIndex,
		Selector:           selector.NewWithStrategy(cfg.Selector.Strategy),
		Health:             healthMirror,
		Emitter:            emitter,
		Creds:              creds,
		MaxRetries:         cfg.Relay.MaxRetries,
		EnableCrossDialect: cfg.Relay.EnableCrossDialect,
	})

	// 主动健康探测（可选，默认关）：独立 goroutine，停机随 ctx 取消。
	var proberCancel context.CancelFunc
	var proberDone <-chan struct{}
	if cfg.Health.Enabled {
		var pctx context.Context
		pctx, proberCancel = context.WithCancel(context.Background())
		defer proberCancel() // 兜底：异常返回路径也取消，防 context 泄漏（正常停机已显式取消并等待）
		prober := health.NewProber(health.Config{
			DAO: dao, Creds: creds, Mark: healthMirror.Mark,
			Interval: cfg.Health.Interval, Timeout: cfg.Health.Timeout,
		})
		proberDone = prober.Run(pctx)
		slog.Info("主动健康探测已启用", "interval", cfg.Health.Interval.String())
	}

	// 保留期清理：启动跑一次 + 每日。stopCleanup 在停机时关闭以结束 goroutine。
	stopCleanup := make(chan struct{})
	runCleanup := func() {
		if cfg.RetentionDays <= 0 {
			return
		}
		cutoff := time.Now().AddDate(0, 0, -cfg.RetentionDays).UTC().Format(time.RFC3339)
		if n, err := statsDAO.CleanupRawLogs(context.Background(), cutoff); err != nil {
			slog.Warn("清理过期请求日志失败", "error", err)
		} else if n > 0 {
			slog.Info("已清理过期请求日志", "deleted", n, "before", cutoff)
		}
	}
	runCleanup()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-stopCleanup:
				return
			case <-ticker.C:
				runCleanup()
			}
		}
	}()

	mcpService := mcp.NewService(mcp.NewDAO(st))

	deps := api.Deps{
		Relay:    api.NewRelayHandler(engine, routeIndex),
		Admin:    api.NewAdminHandler(manager),
		APIKey:   api.NewAPIKeyHandler(manager, keyCache),
		Stats:    api.NewStatsHandler(statsDAO, pricingTable, emitter, dao),
		MCP:      api.NewMCPHandler(mcpService),
		KeyCache: keyCache,
	}

	// 4. 配置热重载（非密钥键）。
	watcher, err := config.Watch(*configPath, func(newCfg *config.Config) {
		// M1：仅热生效日志级别（server 超时、retention 等的实际生效点由后续里程碑接入）。
		setupLogger(newCfg)
	})
	if err != nil {
		// 监听失败不致命：记录后继续（例如配置文件不存在）。
		slog.Warn("启动配置热重载失败，继续运行", "error", err)
	} else {
		defer func() { _ = watcher.Close() }()
	}

	// 5. 构建 HTTP 服务器。
	handler, err := api.NewServer(cfg, st, deps)
	if err != nil {
		return fmt.Errorf("构建 HTTP 服务失败: %w", err)
	}
	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 6. 起服务 + 监听信号优雅停机。
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("HTTP 服务监听中", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	case sig := <-sigCh:
		slog.Info("收到停机信号，开始优雅停机", "signal", sig.String())
	}

	// 优雅停机：先停 HTTP（等在途请求），再由 defer 关闭 store。
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP 服务停机超时/出错", "error", err)
		return err
	}
	// 服务已停（无新请求）：停清理循环，关闭用量发射器并等待采集器做最终 flush。
	close(stopCleanup)
	if proberCancel != nil {
		proberCancel()
		<-proberDone
	}
	emitter.Close()
	<-usageDone
	if dropped := emitter.Dropped(); dropped > 0 {
		slog.Warn("用量事件曾因缓冲满被丢弃", "dropped", dropped)
	}
	slog.Info("proxy-hub 已停止")
	return nil
}

// setupLogger 按配置初始化全局 slog 默认 logger（text 或 json）。
func setupLogger(cfg *config.Config) {
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// isFlag 判断参数是否以 - 开头（flag 而非子命令）。
func isFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}

// dispatchSubcommand 是 CLI 子命令分派。
func dispatchSubcommand(name string, args []string) error {
	switch name {
	case "mcp":
		return runMCPSubcommand(args)
	default:
		return fmt.Errorf("未知子命令: %q", name)
	}
}

// runMCPSubcommand 处理 `proxy-hub mcp sync [--config <path>]`：装配 DB + MCP 服务并全量对账（cron 友好）。
func runMCPSubcommand(args []string) error {
	if len(args) == 0 || args[0] != "sync" {
		return fmt.Errorf("用法: proxy-hub mcp sync [--config <path>]")
	}
	configPath := "config.yaml"
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "--config" {
			configPath = args[i+1]
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	setupLogger(cfg)
	st, err := store.Open(cfg)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer func() { _ = st.Close() }()

	svc := mcp.NewService(mcp.NewDAO(st))
	if err := svc.SyncAll(context.Background()); err != nil {
		return fmt.Errorf("MCP 同步失败: %w", err)
	}
	fmt.Println("MCP 同步完成")
	return nil
}
