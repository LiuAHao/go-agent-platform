#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "  Go Agent Platform - 桌面端启动"
echo "=========================================="

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go，请先安装 Go"
    exit 1
fi

# 检查 Node.js 是否安装
if ! command -v node &> /dev/null; then
    echo "错误: 未找到 Node.js，请先安装 Node.js"
    exit 1
fi

# 构建 Go 后端
echo ""
echo "[1/3] 构建 Go 后端..."
cd "$PROJECT_ROOT"
go build -o go-agent-platform ./cmd/api
echo "  ✓ Go 后端构建完成"

# 安装前端依赖
echo ""
echo "[2/3] 检查前端依赖..."
cd "$PROJECT_ROOT/web/console"
if [ ! -d "node_modules" ]; then
    npm install
fi
echo "  ✓ 前端依赖已就绪"

# 安装桌面端依赖
echo ""
echo "[3/3] 检查桌面端依赖..."
cd "$SCRIPT_DIR"
if [ ! -d "node_modules" ]; then
    npm install
fi
echo "  ✓ 桌面端依赖已就绪"

# 启动前端开发服务器
echo ""
echo "启动前端开发服务器..."
cd "$PROJECT_ROOT/web/console"
npm run dev &
FRONTEND_PID=$!

# 等待前端服务器就绪
sleep 3

# 启动 Electron
echo ""
echo "启动 Electron 桌面端..."
cd "$SCRIPT_DIR"
npm run dev

# 清理
kill $FRONTEND_PID 2>/dev/null || true
