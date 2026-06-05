# 多阶段构建：纯 Go（CGO_ENABLED=0）→ alpine 运行时。

# ---- 构建阶段 ----
FROM golang:1.25-alpine AS build

WORKDIR /src

# 先拷依赖清单以利用缓存。
COPY go.mod go.sum* ./
RUN go mod download

# 拷源码并构建静态二进制；Version 经 ldflags 注入。
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X github.com/huangjunjan/proxy-hub/internal/buildinfo.Version=${VERSION}" \
    -o /out/proxy-hub ./cmd/proxy-hub

# ---- 运行阶段 ----
FROM alpine:3.20

# ca-certificates 供 HTTPS 上游调用；tzdata 供时区。
RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=build /out/proxy-hub /proxy-hub

# 单端口 / 单数据卷。
EXPOSE 7777
VOLUME /data

# 健康检查打到 /healthz（进程 + DB ping）。
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:7777/healthz || exit 1

# 容器内默认数据目录指向卷。
ENV PROXY_HUB_DATA_DIR=/data

ENTRYPOINT ["/proxy-hub"]
