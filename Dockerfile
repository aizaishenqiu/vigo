# 构建阶段
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o vigo .

# 运行阶段
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/vigo .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/app/view ./app/view
COPY --from=builder /app/public ./public
RUN mkdir -p runtime/log
EXPOSE 8080
CMD ["./vigo"]
