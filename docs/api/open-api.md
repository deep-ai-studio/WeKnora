# Open API 对接文档（Partner）

面向外部系统接入 WeKnora 知识库问答能力。对接方使用 **Partner API Key** 调用，无需 WeKnora 用户 JWT；通过 `external_user_id` / `external_session_id` 隔离用户与会话。

---

## 1. 基础信息

| 项 | 说明 |
|---|---|
| Base URL | `https://<your-host>/api/v1`（本地示例：`http://localhost:8080/api/v1`） |
| Partner 认证 | 请求头 `X-Open-API-Key: sk-open-...` |
| 内容类型 | 请求 `Content-Type: application/json` |
| 非流式响应 | `application/json` |
| 流式响应 | `text/event-stream`（SSE） |

---

## 2. 认证与凭证

### 2.1 Partner API Key

- 格式：`sk-open-` 前缀 + 随机字符串
- 传递方式：**仅 HTTP Header**，不支持 Query 参数

```http
X-Open-API-Key: sk-open-xxxxxxxxxxxxxxxx
```

- Key 与租户绑定，且受 **知识库白名单** 约束（只能访问创建 Client 时配置的 `allowed_kb_ids`）
- Key 泄露后由管理员吊销（见 §5），旧 Key 立即失效

### 2.2 凭证由谁创建

Partner Key **不能自助注册**，需 WeKnora 租户管理员在控制台或通过管理接口创建（§5）。创建时返回的明文 Key **只出现一次**，请妥善保存。

---

## 3. 核心接口：问答

### 3.1 端点

```http
POST /api/v1/open/chat
```

### 3.2 请求体

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `external_user_id` | string | 是 | 对接方系统的用户 ID，用于隔离会话与历史 |
| `knowledge_base_id` | string | 是 | 目标知识库 UUID，须在 Client 白名单内 |
| `query` | string | 是 | 用户问题 |
| `external_session_id` | string | 否 | 对接方会话 ID；相同值复用多轮对话 |
| `session_id` | string | 否 | WeKnora 内部 session UUID（高级用法，一般传 `external_session_id` 即可） |
| `mode` | string | 否 | 问答模式，见 §3.4；**默认 `wiki-qa`** |
| `stream` | bool | 否 | `true` 启用 SSE 流式；默认 `false` 一次性 JSON |

**示例（非流式）：**

```json
{
  "external_user_id": "user-10001",
  "external_session_id": "chat-20250607-001",
  "knowledge_base_id": "710c8962-e912-4ac5-ab89-db62f5540345",
  "mode": "wiki-qa",
  "query": "CHN·WU张力卸载缝合技术是什么？"
}
```

### 3.3 问答模式（mode）

| mode | 说明 | 知识库要求 | 行为概要 |
|---|---|---|---|
| `wiki-qa`（默认） | 维基问答，与 WeKnora UI「维基问答」一致 | 须启用 **Wiki** | Agent 多步检索 Wiki 页面（`wiki_search` / `wiki_read_page` 等），可展示思考与工具过程（流式） |
| `rag-qa` | 经典 RAG 文档检索 | 有向量/chunk 索引即可 | 对文档 chunk 检索后直接生成答案；非流式响应含精简引用列表 |

### 3.4 非流式响应（stream: false 或省略）

**HTTP 200**

```json
{
  "success": true,
  "data": {
    "session_id": "a1635c95-fa07-4c83-8c30-6a0757ecc549",
    "external_session_id": "chat-20250607-001",
    "answer": "……最终回答文本……",
    "is_fallback": false,
    "references": []
  }
}
```

| 字段 | 说明 |
|---|---|
| `session_id` | WeKnora 内部会话 ID，可用于后续指定 `session_id` |
| `external_session_id` | 回显对接方会话 ID |
| `answer` | 完整回答 |
| `is_fallback` | RAG 模式下检索弱匹配时为 `true`；wiki-qa 通常不出现 |
| `references` | **仅 `rag-qa` 非流式** 可能返回；为精简引用（约 80 字预览，无 chunk 全文）。**`wiki-qa` 不返回此字段** |

**references 单条结构（rag-qa）：**

```json
{
  "id": "chunk-uuid",
  "knowledge_id": "doc-uuid",
  "knowledge_base_id": "kb-uuid",
  "knowledge_title": "文档标题.pdf",
  "knowledge_filename": "文档标题.pdf",
  "chunk_index": 3,
  "score": 0.85,
  "content_preview": "约 80 字的摘要预览……"
}
```

### 3.5 流式响应（stream: true）

**请求头须包含：**

```http
Accept: text/event-stream
```

**响应：** SSE，`event: message`，每条 `data` 为 JSON，格式与 WeKnora Web 端 Agent 流式一致。

**典型事件类型：**

| response_type | 含义 |
|---|---|
| `agent_query` | 开始处理 |
| `open_api_meta` | Open API 元数据（session_id、external_session_id、mode 等） |
| `thinking` | 思考过程（逐 token） |
| `tool_call` | 工具调用（如 wiki_search） |
| `tool_result` | 工具返回 |
| `answer` | 最终回答（可流式分片） |
| `references` | 引用（rag-qa 流式时可能出现，含完整 chunk；wiki-qa 通常无） |
| `complete` | 本轮结束 |
| `error` | 错误 |

**open_api_meta 示例：**

```json
{
  "response_type": "open_api_meta",
  "done": true,
  "data": {
    "session_id": "……",
    "external_session_id": "chat-20250607-001",
    "assistant_message_id": "……",
    "mode": "wiki-qa",
    "knowledge_base_id": "710c8962-……"
  }
}
```

**curl 示例：**

```bash
curl -sS -N -X POST 'https://<host>/api/v1/open/chat' \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/event-stream' \
  -H 'X-Open-API-Key: sk-open-YOUR_KEY' \
  -d '{
    "external_user_id": "user-10001",
    "external_session_id": "chat-001",
    "knowledge_base_id": "710c8962-e912-4ac5-ab89-db62f5540345",
    "mode": "wiki-qa",
    "stream": true,
    "query": "CHN·WU张力卸载缝合技术是什么？"
  }'
```

> **建议：** 需要进度展示、思考过程、工具调用详情时，优先使用 `stream: true`。wiki-qa 单次非流式可能需 30–60 秒，流式可在数秒内看到首条事件。

### 3.6 多轮对话

1. 首次请求：传 `external_user_id` + 新的 `external_session_id`（或两者都传，session_id 留空）
2. 后续同一对话：保持相同的 `external_user_id` 与 `external_session_id`
3. 新对话：更换 `external_session_id`

WeKnora 会自动映射到内部 Session，并保留该会话内的历史消息（供 Agent 多轮理解）。

---

## 4. 错误码

| HTTP | 场景 |
|---|---|
| 400 | 参数缺失、非法 `mode`、wiki-qa 但 KB 未启用 Wiki |
| 401 | 缺少或无效 `X-Open-API-Key` |
| 403 | `knowledge_base_id` 不在 Client 白名单 |
| 404 | 知识库或 session 不存在 |
| 500 | 服务内部错误 |

错误体示例：

```json
{
  "success": false,
  "error": {
    "code": "FORBIDDEN",
    "message": "knowledge base is not allowed for this client"
  }
}
```

---

## 5. 管理接口（租户管理员）

以下接口使用 **WeKnora JWT**（`Authorization: Bearer <token>`），**不是** Partner Key。供我方运维创建/吊销对接凭证，一般不对 Partner 开放。

### 5.1 创建 Client

```http
POST /api/v1/open-api/clients
Authorization: Bearer <admin-jwt>
X-Tenant-ID: <tenant_id>
Content-Type: application/json
```

```json
{
  "name": "partner-foo",
  "allowed_kb_ids": ["710c8962-e912-4ac5-ab89-db62f5540345"]
}
```

**响应 201：**

```json
{
  "success": true,
  "data": {
    "client": {
      "id": "client-uuid",
      "tenant_id": 10000,
      "name": "partner-foo",
      "allowed_kb_ids": ["710c8962-……"],
      "status": "active",
      "created_at": "……"
    },
    "api_key": "sk-open-ONLY_SHOWN_ONCE"
  }
}
```

### 5.2 列出 Client

```http
GET /api/v1/open-api/clients
Authorization: Bearer <admin-jwt>
```

### 5.3 吊销 Client

```http
POST /api/v1/open-api/clients/{client_id}/revoke
Authorization: Bearer <admin-jwt>
```

---

## 6. 对接建议

### 6.1 选型

| 场景 | 推荐 |
|---|---|
| 医美 Wiki 知识库、需要可解释检索过程 | `mode: wiki-qa` + `stream: true` |
| 仅文档 chunk 检索、要引用摘要 | `mode: rag-qa` + 非流式 |
| 简单后台批处理 | `mode: wiki-qa` + 非流式，注意超时 ≥ 120s |

### 6.2 超时

- 非流式 wiki-qa：建议客户端 **读超时 120s**
- 流式：连接建立后持续读 SSE，直至收到 `complete` 或 `error`

### 6.3 知识边界

- wiki-qa 设计上以 **知识库 Wiki 内容** 为准；若问题与 KB 无关，模型仍可能用通用知识作答（与 Prompt 约束存在差距，对接方 UI 可自行标注「非知识库来源」）
- 问题是否在 KB 中有专页，取决于 Wiki 建设情况；无专页时 Agent 会从相关概念页拼装答案

### 6.4 安全

- Partner Key 仅通过 HTTPS 传输
- 不要将 Key 写入前端页面或公开仓库
- 按 Partner 分配独立 Client，便于吊销与 KB 隔离

---

## 7. 快速自测清单

```bash
# 1. 健康检查
curl -s http://localhost:8080/health

# 2. 非流式 wiki-qa
curl -sS -X POST 'http://localhost:8080/api/v1/open/chat' \
  -H 'Content-Type: application/json' \
  -H 'X-Open-API-Key: sk-open-...' \
  -d '{"external_user_id":"u1","external_session_id":"s1","knowledge_base_id":"<kb-id>","query":"测试问题"}'

# 3. 流式 wiki-qa
curl -sS -N -X POST 'http://localhost:8080/api/v1/open/chat' \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/event-stream' \
  -H 'X-Open-API-Key: sk-open-...' \
  -d '{"external_user_id":"u1","external_session_id":"s2","knowledge_base_id":"<kb-id>","stream":true,"query":"测试问题"}'
```

---

## 8. 变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| Phase 1 | 2025-06 | Partner Key 认证、`/open/chat` 非流式 |
| Phase 1.1 | 2025-06 | 默认 `wiki-qa`；`rag-qa` 可选；references 精简 |
| Phase 1.2 | 2025-06 | `stream: true` SSE 流式，事件格式对齐 Web Agent |

---

## 9. 联系与支持

- 获取 Partner Key、KB ID、环境 Base URL：联系 WeKnora 租户管理员
- 接口 Schema 以部署环境 Swagger 为准（开发环境：`/swagger/index.html`，搜索 `Open API` 标签）
