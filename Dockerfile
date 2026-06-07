# 智策投研 — 服务端 Docker 镜像
# 多阶段构建: Go 编译 → Python 运行时

# ═══ 阶段 1: 编译 Go 服务 ═══
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/server ./cmd/server/

# ═══ 阶段 2: Python 运行时 ═══
FROM python:3.12-slim

# 安装系统依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

# 设置时区
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 安装 Python 采集依赖
COPY scripts/collector/requirements.txt /app/scripts/collector/
RUN pip install --no-cache-dir -r /app/scripts/collector/requirements.txt

# 复制 Go 二进制
COPY --from=builder /app/server /app/server

# 复制 Python 脚本
COPY scripts/ /app/scripts/

# 设置工作目录
WORKDIR /app
ENV APP_ROOT=/app

EXPOSE 8080
CMD ["./server"]
