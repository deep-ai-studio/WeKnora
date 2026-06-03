# WeKnora 医美知识库 — 本地部署指南 (dev-beauty-ai)

## 项目背景

这是基于腾讯 WeKnora (v0.6.0) 定制的医美知识库聊天系统。
- WeKnora 仓库地址: https://github.com/Tencent/WeKnora
- 本分支: `dev-beauty-ai`
- 目标: 将 WeKnora 改造成医美行业专用知识库问答系统

## 已完成的工作

### 1. Qwen 模型接入 (`config/builtin_models.yaml`)
- 配置了 Qwen (DashScope) 作为对话模型和 Embedding 模型
- 对话模型: `qwen-plus`
- 向量模型: `text-embedding-v3` (1024维)
- 使用 OpenAI 兼容协议，通过 `${QWEN_BASE_URL}` 和 `${QWEN_API_KEY}` 环境变量注入

### 2. 医美专用 Prompt 模板
在原有模板基础上新增了 4 个医美模板：

| 模板 ID | 位置 | 用途 |
|---------|------|------|
| `beauty_patient` | system_prompt.yaml | 患者端——面向求美者的咨询助手 |
| `beauty_doctor` | system_prompt.yaml | 医生端——面向执业医师的临床知识助手 |
| `beauty_context` | context_template.yaml | 医美检索上下文拼接格式 |
| `beauty_fallback` | fallback.yaml | 知识库无匹配时的医美友好兜底 |
| `beauty_fallback_prompt` | fallback.yaml | 模型兜底的医美提示词 |

### 3. 默认配置切换 (`config/config.yaml`)
- 默认 system prompt: `default_kb` → `beauty_patient`
- 默认 context template: `default_context` → `beauty_context`
- 默认兜底: `default_fallback_prompt` → `beauty_fallback_prompt`

### 4. Docker 挂载配置 (`docker-compose.yml`)
- 挂载了 `builtin_models.yaml` 使 Qwen 配置生效
- 挂载了定制 prompt 模板文件覆盖镜像内默认版本

---

## 本地启动步骤

### 前置条件
- Docker & Docker Compose
- 至少 6GB 空闲内存
- Qwen API Key (DashScope)

### Step 1: 克隆仓库
```bash
git clone <repo-url>
cd WeKnora
git checkout dev-beauty-ai
```

### Step 2: 创建 .env 文件
```bash
cat > .env << 'EOF'
QWEN_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
QWEN_API_KEY=<你的Qwen API Key>

DB_USER=weknora
DB_PASSWORD=weknora123
DB_NAME=weknora

RETRIEVE_DRIVER=qdrant
QDRANT_COLLECTION=beauty_ai_embeddings

MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin

GIN_MODE=release
WEKNORA_LANGUAGE=zh-CN
TZ=Asia/Shanghai
EOF
```

### Step 3: 如果 Docker Hub 慢，配置国内镜像
```bash
sudo mkdir -p /etc/docker
cat << 'EOF' | sudo tee /etc/docker/daemon.json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://hub-mirror.c.163.com"
  ]
}
EOF
sudo systemctl restart docker
```

### Step 4: 启动服务
```bash
docker-compose --profile qdrant --profile minio up -d
```

### Step 5: 检查服务状态
```bash
docker-compose ps
# 健康检查
curl http://localhost:8080/health
```

### Step 6: 打开 Web 界面
```
浏览器访问: http://localhost:80
```

---

## 新 Session 继续任务

### 告诉 Claude 这些信息:

1. **代码在哪**: `https://github.com/Tencent/WeKnora`, 分支 `dev-beauty-ai`
2. **当前状态**: 已完成 Qwen 模型配置 + 医美 Prompt 模板 + Docker 部署配置，本地已能启动
3. **待办事项**:
   - [ ] 在 WeKnora Web 界面创建「医美知识库」
   - [ ] 上传医美 PDF 文档（通过 Web UI 或 API）
   - [ ] 测试知识库问答效果
   - [ ] 如果需要 PDF 自动解析，docreader 服务已运行在 50051 端口
   - [ ] 可选：将 `beauty_doctor` 设为某个 agent 的默认 prompt（当前默认是 `beauty_patient`）
   - [ ] 可选：配置 Rerank 模型提升检索精度
   - [ ] 可选：对接企业微信等 IM 渠道

### API Key
- Qwen API Key: `sk-e932db49de714b73a9e922e41dfb11d0`
- Qwen Base URL: `https://dashscope.aliyuncs.com/compatible-mode/v1`

---

## 技术架构速览

```
浏览器 (Vue 前端)
    ↓
Go App (:8080) → PostgreSQL (会话/用户/知识库元数据)
    ↓              ↓ Qdrant (向量检索)
    ↓              ↓ Redis (缓存/任务队列)
    ↓              ↓ MinIO (文件存储)
    ↓
Python DocReader (:50051) — PDF/Word/Excel/图片解析
    ↓
Qwen API (DashScope) — Chat / Embedding / Rerank
```

### 对话 Pipeline (事件驱动插件链)
```
LOAD_HISTORY → QUERY_UNDERSTAND → CHUNK_SEARCH_PARALLEL 
→ CHUNK_RERANK → CHUNK_MERGE → FILTER_TOP_K 
→ INTO_CHAT_MESSAGE → CHAT_COMPLETION_STREAM
```

### Prompt 配置位置
- System Prompt: `config/prompt_templates/system_prompt.yaml`
- Context Template: `config/prompt_templates/context_template.yaml`
- Fallback: `config/prompt_templates/fallback.yaml`
- 模型配置: `config/builtin_models.yaml`
- 主配置: `config/config.yaml`
