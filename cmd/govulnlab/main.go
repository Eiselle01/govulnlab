// Command govulnlab 是项目唯一入口，支持三个子命令：
//
//	govulnlab -cmd=serve   # 启动 HTTP 服务（默认）
//	govulnlab -cmd=migrate # 只执行数据库迁移
//	govulnlab -cmd=seed    # 初始化演示数据（幂等）
//
// 单二进制 + 子命令的好处：容器里可以用同一个镜像做 initContainer 迁移，再起服务。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Eiselle01/govulnlab/internal/config"
	"github.com/Eiselle01/govulnlab/internal/handler"
	"github.com/Eiselle01/govulnlab/internal/logger"
	"github.com/Eiselle01/govulnlab/internal/middleware"
	"github.com/Eiselle01/govulnlab/internal/seed"
	"github.com/Eiselle01/govulnlab/internal/service"
	"github.com/Eiselle01/govulnlab/internal/session"
	"github.com/Eiselle01/govulnlab/internal/store"
	"github.com/Eiselle01/govulnlab/internal/webui"
)

func main() {
	cfgPath := flag.String("config", envOr("GVL_CONFIG", "config.yaml"), "配置文件路径")
	cmd := flag.String("cmd", "serve", "子命令：serve | migrate | seed")
	adminUser := flag.String("admin-user", envOr("GVL_SEED_ADMIN_USER", "admin"), "seed：管理员用户名")
	adminPass := flag.String("admin-password", envOr("GVL_SEED_ADMIN_PASSWORD", "Admin@123456"), "seed：管理员密码")
	demoUser := flag.String("demo-user", envOr("GVL_SEED_DEMO_USER", "demo"), "seed：演示用户")
	demoPass := flag.String("demo-password", envOr("GVL_SEED_DEMO_PASSWORD", "Demo@123456"), "seed：演示用户密码")
	flag.Parse()

	if err := run(*cfgPath, *cmd, seed.Options{
		AdminUsername: *adminUser,
		AdminPassword: *adminPass,
		DemoUsername:  *demoUser,
		DemoPassword:  *demoPass,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}

func run(cfgPath, cmd string, seedOpt seed.Options) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.Log)
	if err != nil {
		return err
	}
	slog.SetDefault(log)

	log.Info("配置加载完成",
		"app", cfg.App.Name,
		"env", cfg.App.Env,
		"db", cfg.SafeDSN(),
		"redis", cfg.Redis.Addr,
		"lab_enabled", cfg.Lab.Enabled,
	)

	// 收到 Ctrl+C / SIGTERM 时取消 ctx，触发优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := st.Close(); cerr != nil {
			log.Warn("关闭数据库连接时出错", "err", cerr)
		}
	}()

	switch cmd {
	case "migrate":
		return st.Migrate(ctx, log)

	case "seed":
		if err := st.Migrate(ctx, log); err != nil {
			return err
		}
		res, err := seed.Run(ctx, st, log, seedOpt)
		if err != nil {
			return err
		}
		fmt.Printf("seed 完成，新建 %d 项，跳过 %d 项\n", len(res.Created), len(res.Skipped))
		for _, s := range res.Created {
			fmt.Println("  +", s)
		}
		for _, s := range res.Skipped {
			fmt.Println("  =", s, "（已存在，跳过）")
		}
		return nil

	case "serve":
		if cfg.App.AutoMigrate {
			if err := st.Migrate(ctx, log); err != nil {
				return err
			}
		}
		return serve(ctx, cfg, log, st)

	default:
		return fmt.Errorf("未知子命令 %q（可选：serve / migrate / seed）", cmd)
	}
}

func serve(ctx context.Context, cfg *config.Config, log *slog.Logger, st *store.Store) error {
	tpl, err := webui.New()
	if err != nil {
		return err
	}

	sess := session.NewManager(st.RDB, cfg)
	authSvc := service.NewAuthService(st, log, cfg)
	orderSvc := service.NewOrderService(st, log)
	guestSvc := service.NewGuestbookService(st, log)
	auditSvc := service.NewAuditService(st, log)

	router := handler.NewRouter(&handler.Deps{
		Cfg:       cfg,
		Log:       log,
		Store:     st,
		Sess:      sess,
		Tpl:       tpl,
		Auth:      authSvc,
		Orders:    orderSvc,
		Guestbook: guestSvc,
		Audit:     auditSvc,
	})

	// 全局限流器：按 IP 限速，防止单个来源把服务打满。
	globalLimiter := middleware.NewRateLimiter(st.RDB, log, "global",
		float64(cfg.Security.GlobalRatePerSec), cfg.Security.GlobalBurst)

	// 中间件顺序很关键：恢复要在最外层之一，日志要在能拿到状态码的位置。
	handlerChain := middleware.Chain(router,
		middleware.RequestID,
		middleware.ClientIPMiddleware(cfg.Security.TrustProxy),
		middleware.Recover(log),
		middleware.AccessLog(log),
		middleware.SecurityHeaders,
		middleware.BodyLimit(cfg.App.BodyLimitBytes),
		middleware.LoadSession(sess, log),
		globalLimiter.Middleware(),
	)

	srv := &http.Server{
		Addr:              cfg.App.Addr,
		Handler:           handlerChain,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout(),
		WriteTimeout:      cfg.WriteTimeout(),
		IdleTimeout:       cfg.IdleTimeout(),
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("HTTP 服务已启动", "addr", cfg.App.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	case <-ctx.Done():
		log.Info("收到退出信号，开始优雅关闭", "timeout", cfg.ShutdownTimeout().String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("优雅关闭失败，强制退出", "err", err)
			return err
		}
		log.Info("已完成优雅关闭")
		return nil
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
