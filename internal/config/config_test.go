package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsWithoutFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "not-exist.yaml"))
	if err != nil {
		t.Fatalf("缺少配置文件时应使用默认值启动，实际报错: %v", err)
	}
	if cfg.App.Addr != ":8080" {
		t.Errorf("默认监听地址错误: %s", cfg.App.Addr)
	}
	if cfg.Lab.Enabled != true || cfg.Lab.AllowRemote != false {
		t.Error("靶场默认配置应为 enabled=true / allow_remote=false")
	}
}

func TestLoadFileAndEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
db:
  host: 10.0.0.1
  user: app
  name: testdb
redis:
  addr: 10.0.0.2:6379
lab:
  enabled: false
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GVL_DB_PASSWORD", "secret-from-env")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.DB.Host != "10.0.0.1" || cfg.DB.User != "app" || cfg.DB.Name != "testdb" {
		t.Errorf("配置文件字段未生效: %+v", cfg.DB)
	}
	if cfg.DB.Password != "secret-from-env" {
		t.Errorf("环境变量未覆盖 db.password，实际 %q", cfg.DB.Password)
	}
	if cfg.Lab.Enabled {
		t.Error("lab.enabled=false 未生效")
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "app:secret-from-env@tcp(10.0.0.1:3306)/testdb") {
		t.Errorf("DSN 拼接错误: %s", dsn)
	}
	if strings.Contains(cfg.SafeDSN(), "secret-from-env") {
		t.Error("SafeDSN 不应包含口令")
	}
}

func TestValidateRejectsBadConfig(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("空配置应校验失败")
	}
}
