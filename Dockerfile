# 智策投研 — 服务端 Docker 镜像
# 多阶段构建: 前端构建 → Go 编译 → Python 运行时

# ═══ 阶段 1: 构建前端 ═══
FROM node:22-alpine AS frontend-builder

WORKDIR /build
COPY web-pc/package.json web-pc/package-lock.json ./
RUN npm ci
COPY web-pc/ ./
RUN npm run build

# ═══ 阶段 2: 编译 Go 服务 ═══
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

# 国内镜像加速
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /build
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/server ./cmd/server/

# ═══ 阶段 3: Python 运行时 ═══
FROM python:3.12-slim

# 安装系统依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

# 设置时区
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 安装 Python 采集依赖（国内 pip 镜像）
COPY scripts/collector/requirements.txt /app/scripts/collector/
RUN pip install --no-cache-dir \
    -i https://pypi.tuna.tsinghua.edu.cn/simple \
    -r /app/scripts/collector/requirements.txt

# 复制 Go 二进制
COPY --from=builder /app/server /app/server

# 复制前端构建产物
COPY --from=frontend-builder /build/dist /app/web-dist/

# 复制 Python 脚本
COPY scripts/ /app/scripts/

# 设置工作目录
WORKDIR /app
ENV APP_ROOT=/app

EXPOSE 8080
CMD ["./server"]
