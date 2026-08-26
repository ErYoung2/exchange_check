# 编译阶段
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 设置 Go 代理加速（国内服务器推荐）
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod ./
RUN go mod download

COPY . .

# 编译成无依赖静态二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# 运行阶段（使用极简 alpine 镜像）
FROM alpine:latest

# 安装 ca-certificates 确保能正常发起 HTTPS 请求
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /root/
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]