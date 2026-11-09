.PHONY: help build run test test-race vet fmt tidy lint migrate seed docker-build up down logs smoke clean

# 默认目标是 help：别人 clone 下来先 make 就能看到可用命令。
.DEFAULT_GOAL := help
BINARY := bin/govulnlab

help: ## 显示所有可用目标
	@grep -E '^[a-zA-Z_.-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## 编译二进制到 bin/govulnlab
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/govulnlab

run: ## 本地起服务（需要先准备好 MySQL/Redis）
	go run ./cmd/govulnlab -cmd=serve

test: ## 跑单元测试
	go test ./...

test-race: ## 跑单元测试并开启竞态检测
	go test -race -count=1 ./...

vet: ## 静态检查
	go vet ./...

fmt: ## 格式化
	gofmt -w cmd internal migrations web

tidy: ## 整理依赖
	go mod tidy

lint: vet ## 静态检查入口（CI 与本地一致）

migrate: ## 执行数据库迁移
	go run ./cmd/govulnlab -cmd=migrate

seed: ## 迁移 + 写入演示数据（幂等）
	go run ./cmd/govulnlab -cmd=seed

docker-build: ## 构建镜像
	docker build -t govulnlab:local .

up: ## 起整套依赖与应用
	docker compose up -d --build

down: ## 停止并保留数据
	docker compose down

logs: ## 跟踪应用日志
	docker compose logs -f app

smoke: ## 对已运行的服务做端到端冒烟（Linux/macOS 用 sh 版；Windows 用 scripts/smoke.ps1）
	sh scripts/smoke.sh

clean: ## 清理构建产物
	rm -rf bin
