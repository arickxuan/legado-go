# 仓库规范

## 项目结构与模块组织

本项目是 Go 模块（`gopro`），为 Legado 阅读应用提供书源解析功能。支持 CSS、XPath、JSON、正则和 JavaScript 规则，从网页提取书籍元数据、目录和正文。

```
gopro/
├── reader.go           # 主入口：CLI 模式与 HTTP 模式切换
├── reader_test.go      # 集成测试（依赖真实书源）
├── server/             # Gin HTTP 服务
│   ├── server.go       # 服务初始化、SourceStore 书源管理、路由注册
│   └── handler.go      # 请求/响应结构体、API 处理函数
├── analyzer/           # 规则解析引擎（CSS、XPath、JSON、正则、JS）
├── analyzeurl/         # URL 模板展开
├── model/              # 数据类型：BookSource、Book、BookChapter、Loader
├── webbook/            # 业务流程：搜索、书籍信息、目录、正文
├── tool/               # 调试脚本（非生产代码）
├── rule/               # 规则定义辅助
└── doc/                # 设计文档与书源 JSON 示例
```
## 项目资料 
处理 规则的
doc/yue.md
doc/ding.md
原项目位置 /Users/arick/javaProject/legado，核心的 kotlin 文件是 app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeUrl.kt。

## 构建、测试与运行

```bash
go build ./...                              # 编译全部包
go test -v -run TestLoadSources -timeout 30s # 运行指定测试
go vet ./...                                # 静态分析
gofmt -w .                                  # 格式化代码
```

### 运行方式

```bash
# CLI 模式（默认）
gopro doc/example.json search "关键词"

# HTTP 模式
gopro -mode=http -port=9090 -sources=doc/example.json
```

### HTTP API 接口

| 方法   | 路径               | 说明         |
|--------|-------------------|-------------|
| GET    | /api/sources      | 获取所有书源 |
| POST   | /api/sources      | 批量添加书源 |
| DELETE | /api/sources?url  | 删除指定书源 |
| POST   | /api/search       | 搜索书籍     |
| POST   | /api/bookinfo     | 获取书籍详情 |
| POST   | /api/chapters     | 获取目录     |
| POST   | /api/content      | 获取章节正文 |

## 编码风格与命名规范

- 遵循 Go 标准风格，由 `gofmt` 强制执行：使用 Tab 缩进，左花括号不换行。
- 导出标识符使用 `PascalCase`，未导出使用 `camelCase`。
- JSON 结构体标签必须与 Legado 书源 schema 完全一致（如 `bookSourceName`、`ruleSearch`）。
- 包名使用简短小写单词（`analyzer`、`model`、`webbook`、`server`）。
- 避免使用 `init()` 函数，优先显式初始化。

## 测试规范

- 使用标准 `testing` 包，不依赖外部测试框架。
- 测试函数命名为 `TestXxx`，后缀具有描述性（如 `TestLoadSources`）。
- 集成测试可能访问线上 URL，超时时间设为 30 秒。
- 运行单个测试：`go test -v -run TestName -timeout 30s`

## 提交与 PR 规范

- 提交信息简洁，常用中文，附带 issue 编号：`优化 #5660`、`fix: 描述`。
- PR 需包含：清晰描述、关联 issue、测试结果（控制台输出或截图）。
- 每次提交只包含一个逻辑变更。

## Agent 指令

- 修改分析器规则时，使用 `doc/example.json` 中的示例数据验证。
- `tool/` 目录为调试脚本，修改不影响生产逻辑。
- 提交前必须通过 `go build ./...` 和 `go vet ./...`。
