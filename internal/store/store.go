// Package store 是数据访问层（仓储层）：只负责和 MySQL / Redis 打交道。
//
// 分层约定：
//   - store 不做业务规则判断（不改密码、不校验金额），只做增删改查与错误归一化；
//   - 所有 SQL 一律使用占位符 ?，禁止任何形式的字符串拼接（靶场包除外，且单独成文件）；
//   - 对外暴露 ErrNotFound / ErrDuplicate / ErrNotOwner / ErrInvalidState 四个哨兵错误，
//     让上层不用关心驱动层的 sql.ErrNoRows / 1062。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"github.com/Eiselle01/govulnlab/internal/config"
)

var (
	ErrNotFound     = errors.New("记录不存在")
	ErrDuplicate    = errors.New("记录已存在")
	ErrNotOwner     = errors.New("数据不属于当前用户")
	ErrInvalidState = errors.New("当前状态不允许该操作")
)

// Store 聚合 MySQL 与 Redis 两个客户端。
type Store struct {
	DB  *sql.DB
	RDB *redis.Client
}

// New 建立连接池并做一次 Ping，确保进程启动即暴露依赖问题（fail fast）。
func New(ctx context.Context, cfg *config.Config) (*Store, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.DB.ConnMaxLifetimeMin) * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("数据库 Ping 失败（%s）: %w", cfg.SafeDSN(), err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("Redis Ping 失败（%s）: %w", cfg.Redis.Addr, err)
	}

	return &Store{DB: db, RDB: rdb}, nil
}

func (s *Store) Close() error {
	var firstErr error
	if s.RDB != nil {
		if err := s.RDB.Close(); err != nil {
			firstErr = err
		}
	}
	if s.DB != nil {
		if err := s.DB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Ping 供 /readyz 探针使用：读接口要能真的连上依赖才算 Ready。
func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("mysql: %w", err)
	}
	if err := s.RDB.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

// isDuplicate 把 MySQL 1062（唯一键冲突）翻译成哨兵错误。
func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// execMigrate 记录日志用的小工具，避免迁移逻辑里重复拼日志字段。
func logMigrate(log *slog.Logger, version string) {
	if log != nil {
		log.Info("数据库迁移已应用", "version", version)
	}
}
