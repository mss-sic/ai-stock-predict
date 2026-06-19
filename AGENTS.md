# AGENTS.md — 智策投研 项目开发规范

> 适用于 Codex Agent 自动遵守的开发约束，覆盖 Go 后端 / React 前端 / Python 采集三层。

---

## 1. 本地服务端启动规则

**固定启动流程，禁止使用其他方式，防止端口冲突：**

```bash
# 编译
cd server && go build -o bin/server ./cmd/server/ && cp bin/server server-bin

# 重启
kill $(lsof -ti :8080) 2>/dev/null; sleep 1
launchctl start com.stock.server; sleep 2

# 验证
lsof -ti :8080
```

- **禁止** `go run`、`./server`、`air` 等其他启动方式
- **禁止** 使用 8080 以外的端口
- 前端 Vite HMR 自动热更新，修改前端代码无需重启
- 修改 Python 脚本无需重启服务端

---

## 2. 零硬编码规则 (No Hardcoding)

### 2.1 Go 后端

**所有可变配置必须通过环境变量读取，带 fallback，禁止直接在代码中写字面量：**

```go
// ✅ 正确
func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" { return v }
    return fallback
}
PG_DSN := getEnv("POSTGRES_DSN", "host=localhost ...")

// ❌ 错误
PG_DSN := "host=localhost user=stock password=stock123 dbname=stock_predict"
```

- 数据库 DSN：`POSTGRES_DSN`、`MYSQL_DSN`
- 服务端口：`PORT`（默认 8080）
- AI API Key：`OPENAI_API_KEY`、`OPENAI_BASE_URL`
- 调度表达式：`CRON_EXPR`
- 采集并发数：`COLLECTOR_WORKERS`、`COLLECTOR_CHUNK`

### 2.2 Python 采集脚本

**所有 Python 脚本的数据库连接和其他可变配置必须使用环境变量 + 默认值：**

```python
# ✅ 正确
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

# ❌ 错误  
PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"  # 硬编码
```

- 所有采集脚本统一用 `os.environ.get("PG_DSN", "fallback")`
- API 地址通过环境变量注入
- 禁止在脚本中硬编码绝对路径

### 2.3 前端

**API 请求统一通过 `web-pc/src/services/api.ts` 中的函数，禁止在组件中直接写 URL 字符串：**

```tsx
// ✅ 正确
import { fetchKLine } from '../services/api';
fetchKLine(code)

// ❌ 错误
fetch(`/api/v1/stocks/${code}/kline`)  // 直接硬编码 URL
```

- 所有后端 API 路径集中在 `api.ts` 管理
- 魔法数字（`slice(0,10)`、`pageSize=20` 等）提取为常量
- 第三方 URL（新浪/东财/同花顺）集中在一个常量 Map 中

---

## 3. 错误处理规则

### 3.1 Go 后端

```go
// ✅ 正确：每个 err 都必须处理或显式忽略
result, err := someFunc()
if err != nil {
    log.Printf("[module] operation failed: %v", err)
    response.InternalError(c, "操作失败")
    return
}

// ❌ 错误：吞咽错误
result, _ := someFunc()  // 不检查 err
someFunc()               // 不接收 err
```

- **禁止** 使用 `_` 忽略 error 返回值
- **禁止** 只 `log.Print` 不返回错误给调用方
- Handler 层错误统一用 `response.Error()` 返回
- Repository 层错误向上传递，不在底层静默吞掉

### 3.2 前端

```tsx
// ✅ 正确
try {
    const res = await fetchData();
    setData(res.data?.data || []);
} catch (err) {
    console.error('[Component] fetchData failed:', err);
    showToast('加载失败', 'error');
}

// ❌ 错误：空 catch
try { await fetchData(); } catch {}  // 静默失败
try { await fetchData(); } catch (_) {}  // 静默失败
```

- **禁止** 空 `catch {}` 或 `catch (_) {}`
- 至少 `console.error` 记录错误
- 面向用户的操作需 `showToast` 反馈

### 3.3 Python

```python
# ✅ 正确
try:
    data = fetch_api(code)
except Exception as e:
    log(f"[{code}] API 错误: {e}")
    return None

# ❌ 错误
try:
    data = fetch_api(code)
except:
    pass  # 静默吞掉
```

---

## 4. 数据库 & 性能规则

### 4.1 查询优化

- **禁止** 在循环中执行数据库查询（N+1 问题）
- 批量操作使用 `execute_values` / `CreateInBatches`
- 列表查询必须有分页（默认 `pageSize=20`，最大不超过 `100`）
- 关联查询优先使用 `JOIN LATERAL` 或 `Preload`，避免循环查询

### 4.2 线上版本同步

修改数据库 Schema 时（TABLE、VIEW、INDEX、FUNCTION、TRIGGER、TYPE、ENUM 等一切 SQL 对象）：

**强制规则：所有 SQL 对象必须通过迁移文件创建，禁止任何形式的旁路创建。**

- 在 `server/internal/db/migrations_data.go` 中新增迁移版本
- 同时生成独立修复 SQL → `docs/sql-fixes/YYYY-MM-DD_description.sql`
- 迁移版本号严格递增（v025 → v026 → v027 ...），描述清晰
- **思考**：现有数据如何迁移？是否需要回填脚本？

**Go 模型与 DB 对象一致性检查清单（每次新增/修改 Go struct 时必查）：**
1. 新增 `model/Xxx.go` → 检查是否有对应的 TABLE/VIEW 创建迁移
2. 新增 `gormAutoMigrate(&model.Xxx{})` → 确认字段定义与迁移 SQL 一致
3. Go 中引用了 `db.PG.Table("xxx_view")` → 必须有创建该 VIEW 的迁移
4. Go 中引用了 `db.PG.Raw("SELECT ... FROM yyy")` 中的表/视图 → 迁移中必须存在

**违规示例（已发生）：** `northbound_daily_view` 在 model / service / Python 脚本中多处引用，但迁移中从未创建 → 线上 500 错误。

### 4.3 索引

- 高频查询字段（`code`、`trade_date`、`user_id`）必须有索引
- `uniqueIndex` 用于业务唯一约束
- 复合索引字段顺序：区分度高的在前

---

## 5. 代码组织规则

### 5.1 Go 后端分层

```
handler/   ← HTTP 层（参数校验、响应格式化、调用 service）
service/   ← 业务逻辑层
repository/ ← 数据访问层
model/     ← 数据模型定义
config/    ← 配置管理
collector/  ← 采集调度
```

- **Handler 禁止直接操作数据库**，必须通过 service 或 repository
- **禁止** 单文件超过 800 行，超限拆分为多个文件
- 所有导出函数必须有注释（`// FunctionName does X`）

### 5.2 前端组件

- 页面组件放 `pages/`，可复用组件放 `components/`
- **禁止** 单文件超过 600 行，超限抽取子组件
- 内联样式过多（>10 个 style prop）应提取为 CSS 变量或 `.css` 文件
- `any` 类型使用必须加注释说明原因

### 5.3 Python 脚本

- 每个脚本单一职责（采集/回填/修复 其一）
- 文件头部注释：功能说明 + 数据源 + 使用方法
- 参数通过 argparse 或 `sys.argv` 传入，不硬编码

---

## 6. 注释 & 文档规则

### 6.1 Go

```go
// Package handler provides HTTP handlers for stock data endpoints.

// StockHandler handles stock-related HTTP requests.
type StockHandler struct { svc *service.StockService }

// GetDetail returns full stock detail including basic info and real-time price.
func (h *StockHandler) GetDetail(c *gin.Context) { ... }
```

- 每个 package 至少一行注释
- 每个导出类型/函数必须有 doc comment
- 复杂逻辑块内部加单行注释解释

### 6.2 前端

```tsx
/** Fetches K-line data for the given stock code.
 *  Returns array of { tradeDate, open, close, high, low, volume, amount, turnoverRate }
 */
export const fetchKLine = (code: string) => api.get(`/stocks/${code}/kline`);
```

- 每个 API 函数加 JSDoc 注释
- 复杂 useMemo / useEffect 内部加注释说明依赖和副作用

---

## 7. 健壮性规则

### 7.1 输入校验

- 所有 handler 入口必须校验必填参数（`code`、`id` 非空）
- 数值范围校验（`page` ≥ 1、`pageSize` ≤ 100、`horizon` 1-60）
- 文件上传校验大小和类型

### 7.2 超时与重试

- Python API 请求必须设 `timeout`（默认 30s，不可无限等待）
- Go HTTP 请求设置 context timeout
- 关键采集允许重试 1-2 次，但需要退避

### 7.3 日志规范

```go
log.Printf("[module] action for %s: %v", code, err)   // 统一格式
```

- 格式：`[模块名] 操作描述: 详情`
- 区分 Info / Warn / Error 级别
- 禁止在循环内打印高频日志

---

## 8. 功能完成确认 & 变更日志

- 每次功能对话结束时**主动询问**「功能是否已完成，是否需要归档？」
- 确认后按天更新 `CHANGELOG.md`（`## vX.Y.Z (YYYY-MM-DD)` 格式）
- 条目按功能模块分组
- 用户确认后再 `git commit`
- **禁止**在用户未确认的情况下自动 commit

---

## 9. 发布上线规则

当用户要求发布上线：

1. **前提检查**：`git status` 确认无未提交更改
2. **构建推送**：运行 `./publish.sh`（buildx linux/amd64 → 阿里云 Registry）
3. 仓库地址：`crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest`
4. 输出服务器更新命令：

```bash
docker pull crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest
cd /opt/ai-stock-predict/docker && docker compose up -d
```

---

## 附录：项目架构速查

| 层级 | 目录 | 技术栈 |
|------|------|--------|
| 前端 | `web-pc/` | React 19 + Vite + Arco Design + ReactMarkdown |
| 后端 | `server/` | Go + Gin + GORM (PostgreSQL + MySQL) |
| 采集 | `scripts/collector/` | Python 3.12 + psycopg2 + requests + mootdx |
| 数据库 | PostgreSQL | `stock_predict` 库 |
| 部署 | `docker/` | Docker Compose + Nginx ||

## 8. 前端 UI 风格规范

### 8.1 图标

- **统一使用 `lucide-react`**，禁止 emoji (📊📈) 和原始 Unicode 作为功能图标
- 图标大小规范：页面标题 18-20px、卡片/按钮 14-16px、内联 12px
- 图标颜色优先用 CSS 变量（`'var(--color-text-2)'`）或系统色（`#165DFF`），禁止硬编码杂色
- 常用图标速查：`Settings/User/BarChart3/DollarSign/Coins/Calendar/Clock/Cpu/TrendingUp/ListFilter/CheckCircle/XCircle`

### 8.2 颜色

- **禁止硬编码颜色值**：优先用 CSS 变量体系
  - 文字：`--color-text-1`(主) / `--color-text-2`(次) / `--color-text-3`(辅助)
  - 背景：`--color-bg-1`(基底) / `--color-bg-2`(卡片)
  - 边框：`--color-border-1`(细分) / `--color-border-2`(通常)
  - 填充：`--color-fill-1`(浅灰) / `--color-fill-2`(深灰)
  - 主题：`--color-primary` / `--color-primary-light-*`

### 8.3 卡片规范

- 卡片容器：`background: var(--color-bg-2)` + `border-radius: 10` + `border: 1px solid var(--color-border-2)`
- 卡片 hover：`boxShadow: '0 2px 12px rgba(0,0,0,0.06)'`
- 页头图标：渐变圆角方块背景 + 白色图标，如 `background: 'linear-gradient(135deg, #165DFF, #722ED1)'` + `borderRadius: 10`

### 8.4 表格规范

- 表头：`background: var(--color-fill-1)` + `borderBottom: 2px solid var(--color-border-2)`
- 行 hover：`background: var(--color-fill-1)`
- 行分隔：`borderBottom: 1px solid var(--color-border-1)`
- Badge 标签：`borderRadius: 10` + 半透明彩色背景(`color + '15'`)

### 8.5 字体规范

- 数据/金额用等宽字体：`fontFamily: "'SF Mono', 'Inter', monospace"`
- 页面标题：18px / 700
- 卡片标签：11-12px
- 表格内容：11-12px

### 常用命令速查

```bash
# 前端构建
cd web-pc && npm run build

# 前端开发
cd web-pc && npm run dev

# 后端编译
cd server && go build -o bin/server ./cmd/server/

# 服务重启
kill $(lsof -ti :8080) 2>/dev/null; sleep 1; launchctl start com.stock.server

# 修复单只股票
cd scripts/collector && python3 repair_kline.py <CODE>

# 增量采集
cd scripts/collector && python3 batch_collect.py

# 发布上线
./publish.sh
```
