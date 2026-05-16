<p align="center">
  <h1 align="center">Atom2API</h1>
  <p align="center">OpenAI-compatible proxy for AtomCode CodingPlan</p>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react&logoColor=black" />
  <img src="https://img.shields.io/badge/SQLite-3-003B57?style=for-the-badge&logo=sqlite&logoColor=white" />
  <img src="https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white" />
  <img src="https://img.shields.io/badge/API-OpenAI_Compatible-412991?style=for-the-badge&logo=openai&logoColor=white" />
</p>

<p align="center">
  <b>将 AtomCode CodingPlan 免费模型转为标准 OpenAI 兼容 API，支持 Docker 一键部署。</b>
</p>

<p align="center">
  <a href="#快速开始">快速开始</a> ·
  <a href="#api-接口">API 接口</a> ·
  <a href="#配置">配置</a> ·
  <a href="#部署">部署</a>
</p>

---

## 功能特性

| 功能 | 说明 |
|---|---|
| **OpenAI 兼容** | `/v1/chat/completions` + `/v1/models`，stream 和非 stream 均支持 |
| **Token 管理** | 多 Token 轮询，自动 OAuth 刷新（6h），Admin UI 手动添加 |
| **Admin 面板** | Token / API Key / 用量日志 / 设置，Bootstrap 首次配置 |
| **API Key** | 下游客户端认证，配额控制，过期时间 |
| **Docker 优先** | 单容器 SQLite，零外部依赖，.env 驱动 |
| **轻量** | 3 个 Go 依赖，单二进制 ~15MB，前端内嵌 |

## 可用模型

| 模型 | Provider | Reasoning |
|---|---|:---:|
| `deepseek-v4-flash` | DeepSeek | ✓ |
| `Qwen/Qwen3.6-35B-A3B` | Qwen | ✓ |
| `Qwen/Qwen3-32B` | Qwen | ✓ |
| `Qwen/Qwen3-30B-A3B` | Qwen | ✓ |
| `Qwen/Qwen3-VL-8B-Instruct` | Qwen | ✗ |
| `Qwen/Qwen3-Coder-480B-A35B-Instruct` | Qwen | ✓ |

## 快速开始

### Docker Compose（推荐）

```bash
git clone https://github.com/DeliciousBuding/atom2api.git
cd atom2api

# 配置
cp .env.example .env
# 编辑 .env，填入 ATOMCODE_TOKENS（可选，也可以通过 Admin UI 添加）

# 启动
docker compose up -d

# 访问 http://localhost:8080/admin/ 完成首次配置
```

### 直接运行

```bash
go build -o atom2api .
./atom2api
```

### 使用示例

```bash
# cURL
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'

# Python OpenAI SDK
from openai import OpenAI
client = OpenAI(base_url="http://localhost:8080/v1", api_key="any")
resp = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(resp.choices[0].message.content)
```

## API 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/v1/chat/completions` | Chat Completions（stream + non-stream） |
| `GET` | `/v1/models` | 模型列表 |
| `GET` | `/v1/health` | 健康检查 |
| `POST` | `/api/admin/bootstrap` | 首次设置 Admin Secret |
| `POST` | `/api/admin/auth/login` | Admin 登录 |
| `GET` | `/api/admin/stats` | 统计概览 |
| `GET/POST` | `/api/admin/tokens` | Token CRUD |
| `POST` | `/api/admin/tokens/{id}/refresh` | 强制刷新 Token |
| `GET/POST` | `/api/admin/apikeys` | API Key CRUD |
| `GET` | `/api/admin/usage` | 用量日志 |
| `GET/PUT` | `/api/admin/settings` | 系统设置 |

## 配置

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口 |
| `BIND_ADDR` | `0.0.0.0` | 监听地址 |
| `DB_PATH` | `data/atom2api.db` | SQLite 数据库路径 |
| `ADMIN_SECRET` | 空（首次引导设置） | Admin 密码 |
| `ATOMCODE_TOKENS` | 空 | Token 注入，格式：`token1:refresh1,token2:refresh2` |
| `DEFAULT_MODEL` | `deepseek-v4-flash` | 默认模型 |
| `RATE_LIMIT_RPM` | `0`（不限） | 全局速率限制 |

## 部署

### Docker（推荐）

```yaml
# docker-compose.yml 已内置
services:
  atom2api:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - sqlite-data:/data
    env_file: .env
    restart: unless-stopped
```

### 获取 Token

Token 存储在本地 `~/.atomcode/auth.toml`：

```bash
# Linux/macOS
cat ~/.atomcode/auth.toml

# 输出示例：
# access_token = "sypgszHsjdyMarF6sTpuAL5b"
# refresh_token = "84e168e45b524668a78d115b28bcb371"
```

通过 Admin UI 手动添加，或设置 `ATOMCODE_TOKENS` 环境变量。

## 项目结构

```
atom2api/
├── main.go                 # 入口：路由、启动、优雅关闭
├── config/config.go        # 环境变量加载
├── database/               # SQLite 数据层
├── auth/                   # Token 池、OAuth 刷新、API Key
├── proxy/                  # OpenAI 代理转发、SSE 流式
├── admin/                  # Admin API（Bootstrap + CRUD）
├── middleware/              # CORS、认证、限流、日志
├── frontend/               # React + Tailwind Admin 面板
├── Dockerfile              # 三阶段构建
└── docker-compose.yml      # Docker 部署
```

## License

MIT License
