# 构建阶段
FROM golang:1.27-alpine AS builder

WORKDIR /app

# 设置Go代理
ENV GOPROXY=https://goproxy.cn,direct

# 安装依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dbmanager ./cmd/server

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 复制二进制文件
COPY --from=builder /app/dbmanager .

# 复制前端静态文件
COPY web/ ./web/

# 创建数据目录
RUN mkdir -p /app/data

# 环境变量
ENV PORT=8080

# 暴露端口
EXPOSE 8080

# 数据卷
VOLUME ["/app/data"]

# 启动命令
CMD ["./dbmanager"]
