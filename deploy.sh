#!/bin/bash
#
# WeKnora 一键部署脚本
#
# 用法:
#   chmod +x deploy.sh
#   ./deploy.sh              # 首次部署
#   ./deploy.sh --update     # 更新已有部署（跳过 .env 配置）
#   ./deploy.sh --frontend-only  # 仅重建前端
#   ./deploy.sh --app-only       # 仅重建后端
#
# 环境要求:
#   - Docker + Docker Compose
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
APP_ONLY=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --update)        MODE="update" ;;
        --frontend-only) FRONTEND_ONLY=true ;;
        --app-only)      APP_ONLY=true ;;
        --help|-h)
            echo "用法: $0 [--update|--frontend-only|--app-only]"
            exit 0 ;;
        *) err "未知参数: $1" && exit 1 ;;
    esac
    shift
done

if [ "$FRONTEND_ONLY" = true ] && [ "$APP_ONLY" = true ]; then
    err "--frontend-only 与 --app-only 不能同时使用"
    exit 1
fi

# 本地已有镜像则跳过拉取（Dockerfile 已默认指向 docker.m.daocloud.io）
ensure_image() {
    local ref="$1"
    if docker image inspect "$ref" >/dev/null 2>&1; then
        log "本地已有镜像 ${ref}，跳过拉取"
        return 0
    fi
    log "本地无 ${ref}，从国内源拉取..."
    docker pull "$ref"
}

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
        if ! grep -q "^MAX_FILE_SIZE_MB=" .env; then
            echo "MAX_FILE_SIZE_MB=200" >> .env
            log "已添加 MAX_FILE_SIZE_MB=200"
        fi
    fi
fi

# ──────────────────────────────────────────────
# 3. 启动 Docker 服务
# ──────────────────────────────────────────────
if [ "$FRONTEND_ONLY" = false ] && [ "$APP_ONLY" = false ]; then
    step "启动 Docker 服务"

    log "启动容器（优先使用本地镜像，不强制 pull）..."
    docker compose --profile qdrant --profile minio up -d 2>&1 || {
        err "容器启动失败"
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
    # 4. 注入内置模型到数据库（来自 config/builtin_models.yaml）
    # ──────────────────────────────────────────────
    # 从 .env 读取数据库连接信息和 API 配置
    DB_USER=$(grep '^DB_USER=' .env | cut -d= -f2-)
    DB_NAME=$(grep '^DB_NAME=' .env | cut -d= -f2-)
    QWEN_API_KEY=$(grep '^QWEN_API_KEY=' .env | cut -d= -f2-)
    QWEN_BASE_URL=$(grep '^QWEN_BASE_URL=' .env | cut -d= -f2-)

    # 使用 ON CONFLICT 实现幂等插入，首次部署和后续更新均可用
    inject_model() {
        local mid="$1" mname="$2" mtype="$3" mparams="$4" mis_default="$5"
        log "  注入模型: ${mname} (${mtype})..."
        docker exec WeKnora-postgres psql -U "${DB_USER:-postgres}" -d "${DB_NAME:-WeKnora}" -c "
            INSERT INTO models (id, tenant_id, name, type, source, parameters, is_default, is_builtin, status)
            VALUES ('${mid}', 10000, '${mname}', '${mtype}', 'remote', '${mparams}', ${mis_default}, true, 'active')
            ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name,
                type = EXCLUDED.type,
                parameters = EXCLUDED.parameters,
                is_default = EXCLUDED.is_default,
                is_builtin = EXCLUDED.is_builtin,
                status = EXCLUDED.status,
                deleted_at = NULL;
        " 2>/dev/null
    }

    # LLM 对话模型 (KnowledgeQA)
    inject_model "builtin-qwen-chat" "qwen-plus" "KnowledgeQA" \
        "{\"base_url\": \"${QWEN_BASE_URL}\", \"api_key\": \"${QWEN_API_KEY}\", \"provider\": \"openai\"}" \
        "true"

    # Embedding 向量模型
    inject_model "builtin-qwen-embedding" "text-embedding-v3" "Embedding" \
        "{\"base_url\": \"${QWEN_BASE_URL}\", \"api_key\": \"${QWEN_API_KEY}\", \"provider\": \"openai\", \"embedding_parameters\": {\"dimension\": 1024, \"truncate_prompt_tokens\": 0}}" \
        "true"

    # VLLM 视觉模型（扫描件 OCR）
    inject_model "builtin-qwen-vl" "qwen-vl-plus" "VLLM" \
        "{\"base_url\": \"${QWEN_BASE_URL}\", \"api_key\": \"${QWEN_API_KEY}\", \"provider\": \"openai\"}" \
        "false"

    # Rerank 重排序模型
    inject_model "builtin-qwen-rerank" "gte-rerank" "Rerank" \
        "{\"api_key\": \"${QWEN_API_KEY}\", \"provider\": \"aliyun\"}" \
        "true"

    log "内置模型注入完成（4个模型） ✓"

    # 启用知识库 VLM（找到第一个知识库并绑定视觉模型）
    KB_ID=$(docker exec WeKnora-postgres psql -U "${DB_USER:-postgres}" -d "${DB_NAME:-WeKnora}" -tAc \
        "SELECT id FROM knowledge_bases LIMIT 1;" 2>/dev/null)
    if [ -n "$KB_ID" ]; then
        docker exec WeKnora-postgres psql -U "${DB_USER:-postgres}" -d "${DB_NAME:-WeKnora}" -c "
            UPDATE knowledge_bases SET vlm_config = '{
                \"enabled\": true,
                \"model_id\": \"builtin-qwen-vl\",
                \"model_name\": \"qwen-vl-plus\",
                \"base_url\": \"${QWEN_BASE_URL}\",
                \"api_key\": \"${QWEN_API_KEY}\",
                \"interface_type\": \"openai\"
            }' WHERE id = '${KB_ID}';
        " 2>/dev/null
        log "VLM 模型已绑定到知识库 ✓"
    fi
fi

# ──────────────────────────────────────────────
# 4.5 后端镜像构建 + 部署（Dockerfile，默认国内源）
# ──────────────────────────────────────────────
if [ "$FRONTEND_ONLY" = false ]; then
    step "后端镜像构建与部署"

    ensure_image docker.m.daocloud.io/library/golang:1.26-bookworm
    ensure_image docker.m.daocloud.io/library/debian:12.12-slim

    log "使用 docker/Dockerfile.app 构建后端镜像..."
    docker compose --profile qdrant --profile minio build app 2>&1 || {
        err "后端镜像构建失败"
        exit 1
    }

    log "重建 app 容器..."
    docker compose --profile qdrant --profile minio up -d --no-deps app 2>&1 || {
        err "app 容器启动失败"
        exit 1
    }

    log "等待 app 就绪..."
    for i in $(seq 1 60); do
        if docker exec WeKnora-app curl -sf http://localhost:8080/health &> /dev/null 2>&1; then
            log "app 就绪 ✓"
            break
        fi
        sleep 3
    done

    log "后端镜像部署完成 ✓"
fi

# ──────────────────────────────────────────────
# 5. 前端静态资源构建 + 镜像打包 + 部署
# ──────────────────────────────────────────────
if [ "$APP_ONLY" = false ]; then
    step "前端镜像构建与部署"

    cd "$SCRIPT_DIR"

    # 先构建前端静态资源（需 Node.js）
    if [ -f scripts/build_frontend_dist.sh ]; then
        log "构建前端静态资源..."
        bash scripts/build_frontend_dist.sh 2>&1 || {
            err "前端静态资源构建失败"
            exit 1
        }
    fi

    ensure_image docker.m.daocloud.io/library/nginx:stable-alpine

    log "使用 frontend/Dockerfile 构建前端镜像..."
    docker compose build frontend 2>&1 || {
        err "前端镜像构建失败"
        exit 1
    }

    log "重建 frontend 容器..."
    docker compose up -d --no-deps frontend 2>&1 || {
        err "前端容器启动失败"
        exit 1
    }

    log "前端镜像部署完成 ✓"
fi

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
echo "    ${SCRIPT_DIR}/deploy.sh --app-only       # 仅更新后端"
echo "    ${SCRIPT_DIR}/deploy.sh --frontend-only  # 仅更新前端"
echo ""
