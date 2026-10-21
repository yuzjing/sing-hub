# ./deploy/Dockerfile
# ==========================================
# 阶段 1：前端构建 (纯 Deno 2，原生平台高速打包)
# ==========================================
FROM --platform=$BUILDPLATFORM denoland/deno:latest AS frontend-builder
WORKDIR /app/frontend

COPY frontend/ .

# 构建并标准化产物到 build/ 目录
RUN deno install --node-modules-dir=auto && \
    deno task build && \
    if [ ! -d build ]; then \
      if [ -d dist ]; then cp -r dist build; \
      elif [ -d .svelte-kit/output/client ]; then cp -r .svelte-kit/output/client build; \
      fi; \
    fi && \
    ls -la build/

# ==========================================
# 阶段 2：后端构建 (Go 原生极速交叉编译)
# ==========================================
FROM --platform=$BUILDPLATFORM golang:alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 核心修复 1：清理占位符
RUN rm -rf ./internal/server/dist/*

# 核心修复 2：注意末尾的 /. 代表平铺拷贝所有前端文件，精准覆盖根层级
COPY --from=frontend-builder /app/frontend/build/. ./internal/server/dist/

ARG TARGETARCH
ARG TARGETOS=linux

# 静态交叉编译
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-w -s" -o sing-hub ./cmd/sing-hub

# ==========================================
# 阶段 3：生产运行时镜像 (最终多架构产物)
# ==========================================
FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata openssh-client curl jq
COPY --from=backend-builder /app/sing-hub /usr/local/bin/sing-hub

VOLUME ["/data"]
EXPOSE 8080

ENV DATA_DIR="/data"
CMD ["sing-hub"]