# ./deploy/Dockerfile
# ==========================================
# 阶段 1：前端构建 (使用纯 Deno 2，且强制在原生架构执行，秒级打出前端包)
# ==========================================
FROM --platform=$BUILDPLATFORM denoland/deno:latest AS frontend-builder
WORKDIR /app/frontend

# 复制前端代码
COPY frontend/ .

# 关键参数：强制生成物理 node_modules 树，完美解决 Node-API 插件(如 Rolldown)加载问题
RUN deno install --node-modules-dir=auto && deno task build

# ==========================================
# 阶段 2：后端构建 (利用 Go 原生极速交叉编译)
# ==========================================
FROM --platform=$BUILDPLATFORM golang:alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 将前端纯静态产物塞入 Go Embed 目录
COPY --from=frontend-builder /app/frontend/build ./internal/server/dist

# 接收 Docker Buildx 自动传入的架构参数 (amd64 或 arm64)
ARG TARGETARCH
ARG TARGETOS=linux

# 极致静态交叉编译，脱离 QEMU 的低效模拟
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-w -s" -o sing-hub ./cmd/sing-hub

# ==========================================
# 阶段 3：生产运行时镜像 (自动输出 amd64/arm64 对应镜像)
# ==========================================
FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata openssh-client curl jq
COPY --from=backend-builder /app/sing-hub /usr/local/bin/sing-hub

VOLUME ["/data"]
EXPOSE 8080

ENV DATA_DIR="/data"
CMD ["sing-hub"]