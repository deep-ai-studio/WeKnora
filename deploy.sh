#!/bin/bash
#
# WeKnora 一键部署脚本
#
# 用法:
#   chmod +x deploy.sh
#   ./deploy.sh              # 首次部署
#   ./deploy.sh --update     # 更新已有部署（跳过 .env 配置）
#   ./deploy.sh --frontend-only  # 仅重建前端
#
# 环境要求:
#   - Docker + Docker Compose
#   - Node.js 18+ (用于前端本地构建，绕过 Docker Hub 拉取问题)
#   - pnpm (脚本会自动安装)
#

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC}  $1"; }
err()  { echo -e "${RED}[ERROR]${NC} $1"; }
step() { echo -e "\n${BLUE}==== $1 ====${NC}"; }

# ──────────────────────────────────────────────
# 0. 参数解析
# ──────────────────────────────────────────────
MODE="full"
FRONTEND_ONLY=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --update)        MODE="update" ;;
        --frontend-only) FRONTEND_ONLY=true ;;
        --help|-h)
            echo "用法: $0 [--update|--frontend-only]"
            exit 0 ;;
        *) err "未知参数: $1" && exit 1 ;;
    esac
    shift
done

# ──────────────────────────────────────────────
# 1. 环境检查
# ──────────────────────────────────────────────
step "环境检查"

if ! command -v docker &> /dev/null; then
    err "未找到 Docker，请先安装 Docker"
    exit 1
fi

if ! docker compose version &> /dev/null; then
    err "Docker Compose 不可用，请安装 Docker Compose v2"
    exit 1
fi

log "Docker 环境 OK"

# ──────────────────────────────────────────────
# 2. .env 配置
# ──────────────────────────────────────────────
if [ "$MODE" != "update" ]; then
    step "配置环境变量"

    if [ ! -f ".env" ]; then
        if [ -f ".env.example" ]; then
            cp .env.example .env
            log "已从 .env.example 创建 .env"
            warn "请编辑 .env 文件，填入你的 API Key 等配置后重新运行此脚本"
            warn "至少需要配置: QWEN_BASE_URL, QWEN_API_KEY"
            exit 0
        fi
    else
        log ".env 已存在，跳过"

        # 确保关键配置存在
        if ! grep -q "MAX_FILE_SIZE_MB" .env; then
            echo "MAX_FILE_SIZE_MB=200" >> .env
            log "已添加 MAX_FILE_SIZE_MB=200"
        fi
    fi
fi

# ──────────────────────────────────────────────
# 3. 启动 Docker 服务
# ──────────────────────────────────────────────
if [ "$FRONTEND_ONLY" = false ]; then
    step "启动 Docker 服务"

    # 先尝试 pull 镜像（可能失败）
    log "拉取 Docker 镜像..."
    docker compose pull 2>/dev/null || warn "部分镜像拉取失败（可忽略，将使用本地缓存）"

    # 启动服务（不加 --build，使用已有镜像）
    log "启动容器..."
    docker compose up -d 2>&1 || {
        err "容器启动失败"
        err "如果是 Docker Hub 连接问题，请配置 Docker 镜像加速器："
        err "  /etc/docker/daemon.json 中添加 registry-mirrors"
        exit 1
    }

    # 等待服务就绪
    log "等待 postgres 就绪..."
    for i in $(seq 1 30); do
        if docker exec WeKnora-postgres pg_isready -U weknora &> /dev/null; then
            log "postgres 就绪 ✓"
            break
        fi
        sleep 2
    done

    log "等待 app 就绪..."
    for i in $(seq 1 60); do
        if docker exec WeKnora-app curl -sf http://localhost:8080/health &> /dev/null; then
            log "app 就绪 ✓"
            break
        fi
        sleep 3
    done

    # ──────────────────────────────────────────────
    # 4. 注入 VLM 模型到数据库（如果尚未添加）
    # ──────────────────────────────────────────────
    log "检查 VLM 模型..."
    VLM_EXISTS=$(docker exec WeKnora-postgres psql -U weknora -d weknora -tAc \
        "SELECT COUNT(*) FROM models WHERE id = 'builtin-qwen-vl';" 2>/dev/null || echo "0")

    if [ "$VLM_EXISTS" = "0" ]; then
        log "添加 VLM 视觉模型 (qwen-vl-plus)..."
        QWEN_API_KEY=$(grep QWEN_API_KEY .env | cut -d= -f2)
        QWEN_BASE_URL=$(grep QWEN_BASE_URL .env | cut -d= -f2)

        docker exec WeKnora-postgres psql -U weknora -d weknora -c "
            INSERT INTO models (id, tenant_id, name, type, source, parameters, is_default, is_builtin, status)
            VALUES (
                'builtin-qwen-vl', 10000, 'qwen-vl-plus', 'VLLM', 'remote',
                '{\"api_key\": \"${QWEN_API_KEY}\", \"base_url\": \"${QWEN_BASE_URL}\", \"provider\": \"openai\"}',
                false, true, 'active'
            );
        " 2>/dev/null

        # 启用知识库 VLM（找到第一个知识库）
        KB_ID=$(docker exec WeKnora-postgres psql -U weknora -d weknora -tAc \
            "SELECT id FROM knowledge_bases LIMIT 1;" 2>/dev/null)
        if [ -n "$KB_ID" ]; then
            docker exec WeKnora-postgres psql -U weknora -d weknora -c "
                UPDATE knowledge_bases SET vlm_config = '{
                    \"enabled\": true,
                    \"model_id\": \"builtin-qwen-vl\",
                    \"model_name\": \"qwen-vl-plus\",
                    \"base_url\": \"${QWEN_BASE_URL}\",
                    \"api_key\": \"${QWEN_API_KEY}\",
                    \"interface_type\": \"openai\"
                }' WHERE id = '${KB_ID}';
            " 2>/dev/null
            log "VLM 模型已添加并绑定到知识库 ✓"
        fi
    else
        log "VLM 模型已存在，跳过"
    fi
fi

# ──────────────────────────────────────────────
# 5. 前端本地构建 + 部署
# ──────────────────────────────────────────────
step "前端构建与部署"

cd "$SCRIPT_DIR/frontend"

# 安装 Node 依赖
if [ ! -d "node_modules" ]; then
    log "安装前端依赖..."

    # 确保 pnpm 可用
    if ! command -v pnpm &> /dev/null; then
        log "安装 pnpm..."
        npm install -g pnpm 2>&1 | tail -1
    fi

    pnpm install 2>&1 | tail -3
fi

# 构建生产包
log "构建前端..."
npm run build 2>&1 | tail -5

# 部署到运行中的容器
log "部署前端到容器..."
docker cp dist/. WeKnora-frontend:/usr/share/nginx/html/ 2>/dev/null || {
    err "前端容器未运行，尝试启动..."
    docker compose up -d frontend
    sleep 3
    docker cp dist/. WeKnora-frontend:/usr/share/nginx/html/
}

# 确保 config.js 正确（容器 entrypoint 已生成，这里再确认）
docker exec WeKnora-frontend cat /usr/share/nginx/html/config.js

log "前端部署完成 ✓"

cd "$SCRIPT_DIR"

# ──────────────────────────────────────────────
# 6. 验证
# ──────────────────────────────────────────────
step "部署验证"

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  WeKnora 部署完成！${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  访问地址:   http://$(hostname -I 2>/dev/null || echo 'localhost')"
echo "  前端版本:   简洁模式 (默认开启)"
echo "  上传限制:   200 MB"
echo "  VLM OCR:    qwen-vl-plus"
echo ""
echo "  管理命令:"
echo "    docker compose logs -f app       # 查看后端日志"
echo "    docker compose logs -f frontend  # 查看前端日志"
echo "    docker compose restart app       # 重启后端"
echo "    ${SCRIPT_DIR}/deploy.sh --frontend-only  # 仅更新前端"
echo ""
