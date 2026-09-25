# dbmanager - 数据库连接管理工具

一个基于 **Go + Web** 的轻量级数据库纳管平台，可统一纳管多种常见数据库，通过浏览器完成连接管理、库表管理、SQL 查询、数据操作、**用户权限管理、审计日志、数据库优化**等。服务打包为 Docker 镜像，一键部署。

## ✨ 功能特性

### 核心功能
- **多数据库支持**：MySQL、PostgreSQL、SQLite、MongoDB、Redis 五种数据库统一纳管
- **连接管理**：连接的增删改查，配置持久化存储，一键测试连接（含延迟）
- **连接池**：自动管理连接复用与保活检测
- **库表管理**：
  - MySQL/PostgreSQL 支持库列表展示、快速切换、建库/删库
  - 表列表展示，表级快捷操作（新建、改名、删除，均二次确认）
- **数据操作**：
  - SQL/命令在线执行，结果表格化展示
  - 行级快捷操作：插入、更新（自动生成主键条件的 UPDATE 示例）、删除（二次确认）
  - 索引创建/删除
- **快捷切换**：SQL 编辑器顶部连接快速切换下拉，切换即同步刷新
- **Web UI**：现代化单页应用，左侧连接/库/表导航 + 右侧查询面板

### 权限与安全
- **用户认证**：基于 Token 的会话管理，登录后自动续期，超时自动登出
- **角色体系**：
  - **系统管理员**：启动时从配置文件自动创建，拥有全部权限
  - **普通用户**：注册后需管理员审批才能登录，仅可操作数据库
- **注册审批**：用户注册后状态为待审批，管理员可通过/禁用/删除
- **角色升降级**：管理员可将普通用户设为管理员（保护机制：不能修改自己角色、不能取消最后一个管理员）
- **权限控制**：所有管理接口需管理员权限，所有数据库接口需登录

### 审计与监控
- **审计日志**：所有平台操作（登录、注册、审批、库表操作、SQL 执行、优化操作等）均记录到服务本地文件（JSON Lines 格式），含时间、用户、角色、操作类型、资源、详情、IP、User-Agent
- **在线用户统计**：实时显示当前在线用户数及列表（用户名、角色、登录时间、最后活跃、IP），自动刷新

### 系统配置
- 统一配置文件 `config.json`，可配置：
  - 系统管理员用户名/密码
  - 数据目录、用户文件、审计日志文件路径
  - 会话超时时间

### 数据库专属优化
针对每种数据库的特性提供专属优化操作（工具栏「⚡ 数据库优化」入口）：

| 数据库 | 优化功能 |
|--------|----------|
| **MySQL** | EXPLAIN 执行计划分析、SHOW PROCESSLIST 进程列表 |
| **PostgreSQL** | Schema 管理（建/删/列）、EXPLAIN ANALYZE、VACUUM（FULL/ANALYZE）、ANALYZE、REINDEX、表大小统计、序列列表 |
| **SQLite** | PRAGMA 管理面板（journal_mode/synchronous/foreign_keys/cache_size 等可视化读写）、VACUUM、REINDEX、EXPLAIN QUERY PLAN、版本信息 |
| **MongoDB** | 聚合管道、countDocuments、distinct 去重、集合统计、数据库统计、高级查询（排序/分页/投影） |
| **Redis** | 类型感知 Key 操作（String/Hash/List/Set/ZSet 各自增删改查）、TTL 管理、SCAN 替代 KEYS（生产安全）、服务器 INFO、DBSize、FlushDB |

## 🏗️ 项目结构

```
dbmanager/
├── cmd/server/main.go          # 服务主入口（HTTP 路由）
├── config.json                 # 系统配置文件（管理员账号、数据目录等）
├── internal/
│   ├── model/model.go          # 数据模型定义（用户、审计日志、连接等）
│   ├── config/config.go        # 配置管理（JSON 持久化）
│   ├── db/                     # 数据库驱动层
│   │   ├── manager.go          # 连接池管理器
│   │   ├── mysql.go            # MySQL 驱动
│   │   ├── postgres.go         # PostgreSQL 驱动
│   │   ├── sqlite.go           # SQLite 驱动
│   │   ├── mongo.go            # MongoDB 驱动（含聚合、统计等）
│   │   └── redis.go            # Redis 驱动（含类型感知操作）
│   ├── auth/                   # 认证与用户存储
│   │   ├── store.go            # 用户存储（JSON 文件）
│   │   ├── session.go          # 会话/Token 管理
│   │   └── auth.go             # 登录/注册/登出逻辑
│   ├── audit/audit.go          # 审计日志记录与查询
│   ├── middleware/middleware.go # 认证与权限中间件
│   └── handler/
│       ├── handler.go          # HTTP API 处理器
│       ├── quick.go            # 快捷操作 API（库/表/行/索引）
│       ├── auth.go             # 认证 API
│       ├── admin.go            # 管理 API（用户/审计/在线用户）
│       └── optimize.go         # 数据库专属优化 API
├── web/                        # 前端静态资源
│   ├── index.html
│   ├── style.css
│   └── app.js
├── Dockerfile                  # 多阶段构建镜像
├── docker-compose.yml          # Docker Compose 部署
├── docker-compose-full.yml     # 含测试数据库的一键部署
└── README.md
```

## 🚀 快速开始

### Docker Compose 部署（推荐）

```bash
docker-compose up -d
# 访问 http://localhost:8080
# 默认管理员账号：admin / admin123
```

### Docker 直接运行

```bash
docker build -t dbmanager:latest .
docker run -d --name dbmanager -p 8080:8080 \
  -v dbmanager_data:/app/data \
  dbmanager:latest
```

### 本地开发运行

```bash
go run ./cmd/server
# 或编译
go build -o dbmanager ./cmd/server
./dbmanager
```

### 一键体验（含测试数据库）

`docker-compose-full.yml` 内置 MySQL/PostgreSQL/MongoDB/Redis 测试实例，全部启动后可在 UI 中直接添加连接体验：

```bash
docker-compose -f docker-compose-full.yml up -d
```

| 服务 | 镜像 | 端口 |
|------|------|------|
| dbmanager | 自构建 | 8080 |
| dbmanager-mysql | mysql:8.0 | 13306 |
| dbmanager-postgres | postgres:16-alpine | 15432 |
| dbmanager-mongo | mongo:7 | 37017 |
| dbmanager-redis | redis:7-alpine | 16379 |

## ⚙️ 系统配置

通过 `config.json` 配置系统参数：

```json
{
  "port": "8080",
  "data_dir": "data",
  "connections_file": "data/connections.json",
  "users_file": "data/users.json",
  "audit_log_file": "data/audit.log",
  "session_timeout_minutes": 120,
  "admin": {
    "username": "admin",
    "password": "admin123"
  }
}
```

## 📦 支持的数据库

| 数据库 | 连接测试 | 库管理 | 表管理 | 行增删改查 | 索引 | 专属优化 |
|--------|:---:|:---:|:---:|:---:|:---:|:---:|
| MySQL | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| PostgreSQL | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| SQLite | ✅ | — | ✅ | ✅ | ✅ | ✅ |
| MongoDB | ✅ | — | ✅ | ✅ | ✅ | ✅ |
| Redis | ✅ | — | — | ✅ (Key) | — | ✅ |

## 🔌 API 接口

### 认证与用户

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| POST | `/api/auth/login` | 登录 | 公开 |
| POST | `/api/auth/register` | 注册（待审批） | 公开 |
| POST | `/api/auth/logout` | 登出 | 登录 |
| GET | `/api/auth/me` | 当前用户信息 | 登录 |

### 管理（需管理员）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/users` | 用户列表 |
| PUT | `/api/admin/users/{id}/approve` | 审批用户 |
| PUT | `/api/admin/users/{id}/status` | 启用/禁用用户 |
| PUT | `/api/admin/users/{id}/role` | 修改角色 |
| DELETE | `/api/admin/users/{id}` | 删除用户 |
| GET | `/api/admin/audit-logs` | 审计日志 |
| GET | `/api/admin/online-users` | 在线用户 |

### 连接与查询

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/connections` | 获取所有连接 |
| POST | `/api/connections` | 新增连接 |
| PUT | `/api/connections/{id}` | 更新连接 |
| DELETE | `/api/connections/{id}` | 删除连接 |
| POST | `/api/connections/test` | 测试连接 |
| GET | `/api/connections/{id}/tables` | 获取表列表 |
| POST | `/api/connections/query` | 执行查询 |

### 快捷操作

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/quick/{id}/databases` | 库列表 |
| GET | `/api/quick/{id}/tables?db=` | 指定库的表列表 |
| GET | `/api/quick/{id}/columns?table=` | 表列信息（含主键） |
| POST | `/api/quick/{id}/table` | 新建表 |
| PUT | `/api/quick/{id}/table/rename` | 表改名 |
| DELETE | `/api/quick/{id}/table` | 删除表 |
| GET | `/api/quick/{id}/rows` | 行数据浏览 |
| POST | `/api/quick/{id}/row` | 插入行 |
| PUT | `/api/quick/{id}/row` | 更新行 |
| DELETE | `/api/quick/{id}/row` | 删除行 |
| GET | `/api/quick/{id}/indexes?table=` | 索引列表 |
| POST | `/api/quick/{id}/index` | 创建索引 |
| DELETE | `/api/quick/{id}/index` | 删除索引 |

### 数据库优化

| 方法 | 路径 | 说明 | 适用 |
|------|------|------|------|
| GET | `/api/quick/{id}/redis/keys` | Key 列表（SCAN） | Redis |
| GET | `/api/quick/{id}/redis/key` | Key 详情（类型感知） | Redis |
| POST | `/api/quick/{id}/redis/key` | 设置 String | Redis |
| PUT | `/api/quick/{id}/redis/ttl` | 设置过期时间 | Redis |
| DELETE | `/api/quick/{id}/redis/key` | 删除 Key | Redis |
| GET | `/api/quick/{id}/redis/info` | 服务器信息 | Redis |
| GET | `/api/quick/{id}/redis/dbsize` | Key 数量 | Redis |
| POST | `/api/quick/{id}/redis/flushdb` | 清空当前 DB | Redis |
| POST | `/api/quick/{id}/mongo/aggregate` | 聚合管道 | MongoDB |
| GET | `/api/quick/{id}/mongo/count` | 文档统计 | MongoDB |
| GET | `/api/quick/{id}/mongo/distinct` | 去重查询 | MongoDB |
| GET | `/api/quick/{id}/mongo/stats/collection` | 集合统计 | MongoDB |
| GET | `/api/quick/{id}/mongo/stats/database` | 数据库统计 | MongoDB |
| GET | `/api/quick/{id}/pg/schemas` | Schema 列表 | PostgreSQL |
| POST | `/api/quick/{id}/pg/schema` | 创建 Schema | PostgreSQL |
| DELETE | `/api/quick/{id}/pg/schema` | 删除 Schema | PostgreSQL |
| POST | `/api/quick/{id}/pg/explain` | EXPLAIN ANALYZE | PostgreSQL |
| POST | `/api/quick/{id}/pg/vacuum` | VACUUM | PostgreSQL |
| POST | `/api/quick/{id}/pg/analyze` | ANALYZE | PostgreSQL |
| POST | `/api/quick/{id}/pg/reindex` | REINDEX | PostgreSQL |
| GET | `/api/quick/{id}/pg/sizes` | 表大小统计 | PostgreSQL |
| GET | `/api/quick/{id}/pg/sequences` | 序列列表 | PostgreSQL |
| GET | `/api/quick/{id}/sqlite/pragmas` | PRAGMA 列表 | SQLite |
| PUT | `/api/quick/{id}/sqlite/pragma` | 设置 PRAGMA | SQLite |
| POST | `/api/quick/{id}/sqlite/vacuum` | VACUUM | SQLite |
| POST | `/api/quick/{id}/sqlite/reindex` | REINDEX | SQLite |
| POST | `/api/quick/{id}/sqlite/explain` | EXPLAIN QUERY PLAN | SQLite |
| GET | `/api/quick/{id}/sqlite/version` | SQLite 版本 | SQLite |

## 🛠️ 技术栈

- **后端**：Go (net/http 标准库，无第三方框架依赖)
- **驱动**：go-sql-driver/mysql、lib/pq、modernc.org/sqlite、mongo-driver、go-redis
- **前端**：原生 HTML/CSS/JavaScript
- **部署**：Docker 多阶段构建（alpine 运行时），数据卷持久化
- **存储**：用户/连接/审计日志均以 JSON 文件持久化到数据目录
