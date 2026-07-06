# Build stage
FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/golang:1.26.4-alpine AS builder

WORKDIR /app

# 设置 Go 代理环境变量
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

# Copy go mod files
COPY go.mod go.sum ./

# Copy source code
COPY . .

# Download dependencies
RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o kubegraph ./backend

# Final stage
FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/alpine:3.20.2

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/kubegraph /kubegraph

USER root

# Command to run when container starts
CMD ["/kubegraph"]
