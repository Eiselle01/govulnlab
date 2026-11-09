// Package config 负责加载、校验、暴露全局配置。
//
// 设计要点：
//  1. 所有配置项都有默认值（viper.SetDefault），因此“零配置文件 + 纯环境变量”也能启动，
//     这对容器化部署非常关键；
//  2. 环境变量统一前缀 GVL_，层级用下划线连接（GVL_DB_PASSWORD -> db.password）；
//  3. 敏感信息（数据库口令）不写进仓库，用环境变量注入，见 .env.example；
//  4. 校验失败直接启动失败（fail fast），避免带着错配置跑到线上才发现。
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 是应用的完整配置树，yaml 字段与 mapstructure tag 一一对应。
type Config struct {
	App      App      `mapstructure:"app"`
	Log      Log      `mapstructure:"log"`
	DB       DB       `mapstructure:"db"`
	Redis    Redis    `mapstructure:"redis"`
	Session  Session  `mapstructure:"session"`
	Security Security `mapstructure:"security"`
	Lab      Lab      `mapstructure:"lab"`
}

type App struct {
	Name            string `mapstructure:"name"`
	Env             string `mapstructure:"env"`
	Addr            string `mapstructure:"addr"`
	ReadTimeoutSec  int    `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int    `mapstructure:"write_timeout_sec"`
	IdleTimeoutSec  int    `mapstructure:"idle_timeout_sec"`
	ShutdownSec     int    `mapstructure:"shutdown_sec"`
	BodyLimitBytes  int64  `mapstructure:"body_limit_bytes"`
	AutoMigrate     bool   `mapstructure:"auto_migrate"`
}

type Log struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type DB struct {
	Host               string `mapstructure:"host"`
	Port               int    `mapstructure:"port"`
	User               string `mapstructure:"user"`
	Password           string `mapstructure:"password"`
	Name               string `mapstructure:"name"`
	MaxOpenConns       int    `mapstructure:"max_open_conns"`
	MaxIdleConns       int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeMin int    `mapstructure:"conn_max_lifetime_min"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type Session struct {
	CookieName string `mapstructure:"cookie_name"`
	TTLMinutes int    `mapstructure:"ttl_minutes"`
	Secure     bool   `mapstructure:"secure"`
}

type Security struct {
	LoginMaxFail      int  `mapstructure:"login_max_fail"`
	LoginBlockMinutes int  `mapstructure:"login_block_minutes"`
	PasswordMinLen    int  `mapstructure:"password_min_len"`
	GlobalRatePerSec  int  `mapstructure:"global_rate_per_sec"`
	GlobalBurst       int  `mapstructure:"global_burst"`
	TrustProxy        bool `mapstructure:"trust_proxy"`
}

// Lab 控制“故意留洞”的靶场模块。默认只允许本机访问，防止被公网扫描器当成肉鸡。
type Lab struct {
	Enabled     bool `mapstructure:"enabled"`
	AllowRemote bool `mapstructure:"allow_remote"`
}

// Load 读取配置文件并叠加环境变量。
// 配置文件不存在时不报错：允许完全靠环境变量启动（容器场景）。
func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix("GVL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path == "" {
		path = "config.yaml"
	}
	if _, err := os.Stat(path); err == nil {
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("访问配置文件 %s 失败: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "govulnlab")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.addr", ":8080")
	v.SetDefault("app.read_timeout_sec", 15)
	v.SetDefault("app.write_timeout_sec", 30)
	v.SetDefault("app.idle_timeout_sec", 60)
	v.SetDefault("app.shutdown_sec", 10)
	v.SetDefault("app.body_limit_bytes", 1<<20) // 1MB
	v.SetDefault("app.auto_migrate", true)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

	v.SetDefault("db.host", "127.0.0.1")
	v.SetDefault("db.port", 3306)
	v.SetDefault("db.user", "root")
	v.SetDefault("db.password", "")
	v.SetDefault("db.name", "govulnlab")
	v.SetDefault("db.max_open_conns", 25)
	v.SetDefault("db.max_idle_conns", 10)
	v.SetDefault("db.conn_max_lifetime_min", 30)

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)

	v.SetDefault("session.cookie_name", "gvl_session")
	v.SetDefault("session.ttl_minutes", 120)
	v.SetDefault("session.secure", false)

	v.SetDefault("security.login_max_fail", 5)
	v.SetDefault("security.login_block_minutes", 15)
	v.SetDefault("security.password_min_len", 10)
	v.SetDefault("security.global_rate_per_sec", 20)
	v.SetDefault("security.global_burst", 40)
	v.SetDefault("security.trust_proxy", false)

	v.SetDefault("lab.enabled", true)
	v.SetDefault("lab.allow_remote", false)
}

// Validate 做启动前的强校验，宁可起不来也不要带病运行。
func (c *Config) Validate() error {
	if strings.TrimSpace(c.App.Addr) == "" {
		return fmt.Errorf("app.addr 不能为空")
	}
	if c.DB.Host == "" || c.DB.User == "" || c.DB.Name == "" {
		return fmt.Errorf("db.host/db.user/db.name 不能为空")
	}
	if c.DB.Port < 1 || c.DB.Port > 65535 {
		return fmt.Errorf("db.port 非法: %d", c.DB.Port)
	}
	if c.DB.MaxOpenConns < 1 || c.DB.MaxIdleConns < 0 {
		return fmt.Errorf("数据库连接池参数非法: max_open=%d max_idle=%d", c.DB.MaxOpenConns, c.DB.MaxIdleConns)
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis.addr 不能为空")
	}
	if c.Session.CookieName == "" {
		return fmt.Errorf("session.cookie_name 不能为空")
	}
	if c.Session.TTLMinutes < 1 {
		return fmt.Errorf("session.ttl_minutes 必须 >= 1")
	}
	if c.Security.PasswordMinLen < 8 || c.Security.PasswordMinLen > 72 {
		return fmt.Errorf("security.password_min_len 需在 8~72（bcrypt 上限 72 字节）")
	}
	if c.Security.GlobalRatePerSec < 1 || c.Security.GlobalBurst < 1 {
		return fmt.Errorf("限流参数非法")
	}
	if c.App.BodyLimitBytes < 1024 {
		return fmt.Errorf("app.body_limit_bytes 太小: %d", c.App.BodyLimitBytes)
	}
	return nil
}

// DSN 拼 MySQL DSN。parseTime=true 让 DATETIME 直接扫描成 time.Time，
// loc=Local 保证时区语义与数据库一致。
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local&timeout=3s&readTimeout=10s&writeTimeout=10s",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name,
	)
}

// SafeDSN 用于日志/启动横幅，隐藏口令。
func (c *Config) SafeDSN() string {
	return fmt.Sprintf("%s@tcp(%s:%d)/%s", c.DB.User, c.DB.Host, c.DB.Port, c.DB.Name)
}

func (c *Config) ReadTimeout() time.Duration {
	return time.Duration(c.App.ReadTimeoutSec) * time.Second
}
func (c *Config) WriteTimeout() time.Duration {
	return time.Duration(c.App.WriteTimeoutSec) * time.Second
}
func (c *Config) IdleTimeout() time.Duration {
	return time.Duration(c.App.IdleTimeoutSec) * time.Second
}
func (c *Config) ShutdownTimeout() time.Duration {
	return time.Duration(c.App.ShutdownSec) * time.Second
}
func (c *Config) SessionTTL() time.Duration { return time.Duration(c.Session.TTLMinutes) * time.Minute }
func (c *Config) LoginBlockDuration() time.Duration {
	return time.Duration(c.Security.LoginBlockMinutes) * time.Minute
}
