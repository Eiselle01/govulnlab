# syntax=docker/dockerfile:1

# ---------- 构建阶段 ----------
# 用固定版本的基础镜像，避免“今天能构建、明天不能”的不确定性。
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# 先只拷贝依赖清单并下载：只要 go.mod/go.sum 没变，这一层就能命中缓存。
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 → 静态链接，运行镜像不需要任何 libc；
# -trimpath 去掉本机绝对路径（既减小体积，也避免把开发机目录结构泄露到镜像里）。
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/govulnlab ./cmd/govulnlab

# ---------- 运行阶段 ----------
FROM alpine:3.21

# ca-certificates：将来接外部 HTTPS 服务时需要；
# tzdata：容器内日志与审计时间要能和宿主机对上。
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app

COPY --from=build /out/govulnlab /usr/local/bin/govulnlab
COPY config.yaml /etc/govulnlab/config.yaml

# 以非 root 运行：容器逃逸的影响面会小很多。
USER app

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/govulnlab"]
CMD ["-config", "/etc/govulnlab/config.yaml", "-cmd", "serve"]
