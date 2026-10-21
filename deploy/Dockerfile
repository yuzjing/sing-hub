# ./deploy/Dockerfile
# ==========================================
# 阶段 1：前端构建 (采用最新 Deno)
# ==========================================
FROM denoland/deno:alpine AS frontend-builder
WORKDIR /app/frontend
# 复制前端源码
COPY frontend/ .
# 安装依赖并构建静态文件
RUN deno install && deno task build

# ==========================================
# 阶段 2：后端构建 (采用最新 Go)
# ==========================================
FROM golang:alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 将前端打包后的产物放入 Go Embed 目录
COPY --from=frontend-builder /app/frontend/build ./internal/server/dist

# 极致压缩编译核心引擎
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o sing-hub ./cmd/sing-hub

# ==========================================
# 阶段 3：生产运行时镜像 (Alpine Latest)
# ==========================================
FROM alpine:latest
WORKDIR /app

# 安装必要的系统组件
# curl/jq: 用于后续 Go 运行时自动拉取 sing-box 官方二进制
# git: 用于本地仓库版本快照管理
# openssh-client: 用于远程节点部署推送
# tzdata: 时区支持
RUN apk add --no-cache git ca-certificates tzdata openssh-client curl jq

# 仅复制我们自己编译出的二进制核心
COPY --from=backend-builder /app/sing-hub /usr/local/bin/sing-hub

# 持久化数据目录
VOLUME ["/data"]
EXPOSE 8080

# 环境变量设定 (指向挂载的持久化目录，方便存放下载的二进制文件)
ENV DATA_DIR="/data"
ENV SINGBOX_BIN_DIR="/data/bin"

CMD ["sing-hub"]