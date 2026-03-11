.PHONY: help build run test clean swagger docker-build docker-up docker-down docker-logs deploy

APP_NAME = vigo
VERSION = 2.0.12

help:
	@echo "Vigo 框架 - 可用命令:"
	@echo ""
	@echo "  开发命令:"
	@echo "    build        - 编译项目"
	@echo "    run          - 编译并运行"
	@echo "    test         - 运行测试"
	@echo "    clean        - 清理编译文件"
	@echo "    swagger      - 生成 Swagger 文档"
	@echo ""
	@echo "  Docker 命令:"
	@echo "    docker-build - 构建 Docker 镜像"
	@echo "    docker-up    - 启动所有服务"
	@echo "    docker-down  - 停止所有服务"
	@echo "    docker-logs  - 查看服务日志"
	@echo ""
	@echo "  部署命令 (Linux):"
	@echo "    deploy       - 部署到服务器"
	@echo ""

build:
	@echo "正在编译 $(APP_NAME)..."
	go build -ldflags="-s -w" -o $(APP_NAME) .
	@echo "编译完成: $(APP_NAME)"

run: build
	@echo "正在启动 $(APP_NAME)..."
	./$(APP_NAME)

test:
	@echo "正在运行测试..."
	go test -v ./...

clean:
	@echo "正在清理..."
	rm -f $(APP_NAME)
	rm -f tmp/*.exe
	rm -f tmp/*.log
	@echo "清理完成"

swagger:
	@echo "正在生成 Swagger 文档..."
	swag init
	@echo "Swagger 文档已生成"

docker-build:
	@echo "正在构建 Docker 镜像..."
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .
	@echo "Docker 镜像构建完成"

docker-up:
	@echo "正在启动 Docker 服务..."
	docker-compose up -d
	@echo "服务已启动"
	@echo "查看日志: make docker-logs"

docker-down:
	@echo "正在停止 Docker 服务..."
	docker-compose down
	@echo "服务已停止"

docker-logs:
	docker-compose logs -f

deploy: build
	@echo "正在部署..."
	chmod +x deploy.sh
	./deploy.sh
