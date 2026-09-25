# dbmanager - 数据库连接管理工具

一个基于 **Go + Web** 的轻量级数据库纳管平台，可统一纳管多种常见数据库，通过浏览器完成连接管理、库表管理、SQL 查询与数据操作。服务打包为 Docker 镜像，一键部署。

## ✨ 功能特性

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

## 🏗️ 项目结构

```
dbmanager/
├── cmd/server/main.go          # 服务主入口（HTTP 路由）
├── internal/
│   ├── model/model.go          # 数据模型定义
│   ├── config/config.go        # 配置管理（JSON 持久化）
│   ├── db/                     # 数据库驱动层
│   │   ├── manager.go          # 连接池管理器
│   │   ├── mysql.go            # MySQL 驱动
│   │   ├── postgres.go         # PostgreSQL 驱动
│   │   ├── sqlite.go           # SQLite 驱动
│   │   ├── mongo.go            # MongoDB 驱动
│   │   └── redis.go            # Redis 驱动
│   └── handler/
│       ├── handler.go          # HTTP API 处理器
│       └── quick.go            # 快捷操作 API（库/表/行/索引）
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

## 📦 支持的数据库

| 数据库 | 连接测试 | 库管理 | 表管理 | 行增删改查 | 索引 |
|--------|:---:|:---:|:---:|:---:|:---:|
| MySQL | ✅ | ✅ | ✅ | ✅ | ✅ |
| PostgreSQL | ✅ | ✅ | ✅ | ✅ | ✅ |
| SQLite | ✅ | — | ✅ | ✅ | ✅ |
| MongoDB | ✅ | — | ✅ | ✅ | ✅ |
| Redis | ✅ | — | — | ✅ (Key) | — |

## 🔌 API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/connections` | 获取所有连接 |
| POST | `/api/connections` | 新增连接 |
| PUT | `/api/connections/{id}` | 更新连接 |
| DELETE | `/api/connections/{id}` | 删除连接 |
| POST | `/api/connections/test` | 测试连接 |
| GET | `/api/connections/{id}/tables` | 获取表列表 |
| POST | `/api/connections/query` | 执行查询 |
| GET | `/api/quick/{id}/databases` | 库列表 |
| GET | `/api/quick/{id}/tables?db=` | 指定库的表列表 |
| GET | `/api/quick/{id}/columns?table=` | 表列信息（含主键） |
| POST | `/api/quick/{id}/table` | 新建表 |
| PUT | `/api/quick/{id}/table/rename` | 表改名 |
| DELETE | `/api/quick/{id}/table` | 删除表 |
| GET | `/api/quick/{id}/rows` | 行数据浏览 |
| POST | `/api/quick/{id}/row` | 插入行 |
| POST | `/api/quick/{id}/row/update` | 更新行 |
| POST | `/api/quick/{id}/row/delete` | 删除行 |
| GET | `/api/quick/{id}/indexes?table=` | 索引列表 |
| POST | `/api/quick/{id}/index` | 创建索引 |
| DELETE | `/api/quick/{id}/index` | 删除索引 |

## 🛠️ 技术栈

- **后端**：Go (net/http 标准库，无第三方框架依赖)
- **驱动**：go-sql-driver/mysql、pgx、mattn/go-sqlite3、mongo-driver、go-redis
- **前端**：原生 HTML/CSS/JavaScript
- **部署**：Docker 多阶段构建（alpine 运行时），数据卷持久化
