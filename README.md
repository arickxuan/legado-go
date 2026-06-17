# GoPro — Legado 书源引擎 (Go 实现)

GoPro 是 [Legado](https://github.com/gedoor/legado) 阅读应用书源解析规则的 **Go 语言实现**。它可以从网页中提取书籍元数据、目录和正文，支持 CSS 选择器、XPath、JSONPath、正则表达式和 JavaScript 等多种规则引擎。

> **Legado**（又名「开源阅读」）是一款 Android 端网络小说阅读器，其核心能力是通过用户编写的「书源」JSON 规则，从任意网站抓取和解析小说内容。GoPro 将该能力迁移到 Go 平台，可独立作为 CLI 工具或 HTTP 服务使用。

---

## 特性

- ✨ **完整支持 Legado 书源规则** — 搜索、书籍详情、目录、正文、发现
- 🧩 **多种规则引擎** — CSS 选择器、XPath、JSONPath、正则表达式、JavaScript (QuickJS)
- 🌐 **双重运行模式** — CLI 命令行模式 & HTTP API 服务模式
- 🔧 **JavaScript 沙箱** — 内置 QuickJS 引擎，支持书源中的 JS 自定义逻辑
- 📦 **自包含** — 单个二进制文件，无外部依赖
- 🧪 **可测试** — 支持诊断模式，便于调试书源规则

---

## 快速开始

### 安装

```bash
git clone <repo-url> && cd gopro
go build -o gopro ./...
```

### 运行模式

#### CLI 模式（默认）

```bash
# 搜索书籍
./gopro doc/example.json search "斗破苍穹"

# 输出 JSON 格式搜索结果
```

#### HTTP 服务模式

```bash
./gopro -mode=http -port=9090 -sources=doc/example.json
```

启动后可通过 `http://localhost:9090/api/` 访问 API。

---

## HTTP API

| 方法   | 路径               | 说明             |
|--------|-------------------|-----------------|
| GET    | `/api/sources`    | 获取所有书源       |
| POST   | `/api/sources`    | 批量添加书源       |
| DELETE | `/api/sources?url`| 删除指定书源       |
| POST   | `/api/search`     | 搜索书籍           |
| POST   | `/api/bookinfo`   | 获取书籍详情       |
| POST   | `/api/chapters`   | 获取目录列表       |
| POST   | `/api/content`    | 获取章节正文       |

### API 请求示例

**搜索书籍：**

```bash
curl -X POST http://localhost:9090/api/search \
  -H "Content-Type: application/json" \
  -d '{"keyword": "斗破苍穹", "page": 1}'
```

**获取章节正文：**

```bash
curl -X POST http://localhost:9090/api/content \
  -H "Content-Type: application/json" \
  -d '{
    "sourceUrl": "https://www.example.com",
    "book": {"bookUrl": "https://www.example.com/book/123", "name": "...", ...},
    "chapter": {"url": "https://www.example.com/chapter/1", "title": "第一章"}
  }'
```

---

## 支持的规则类型

GoPro 完整支持 Legado 书源所使用的规则语法：

| 规则类型        | 前缀 / 标识               | 说明                        |
|----------------|---------------------------|-----------------------------|
| **CSS 选择器**  | `@css:` 或 `@@`           | 基于 goquery 的 CSS 解析     |
| **XPath**       | `@xpath:` 或 `//`         | 基于 htmlquery 的 XPath 解析 |
| **JSONPath**    | `@json:` 或 `$.` / `$[`   | 基于 ojg 的 JSON 取值        |
| **正则表达式**   | `:` 前缀或规则含 `##`      | 正则匹配与替换               |
| **JavaScript**  | `@js:` 或 `<js>...</js>`  | QuickJS 沙箱执行             |
| **字符串拼接**   | `&&`                      | 多个规则结果拼接             |
| **列表交错**     | `%%`                      | 多个规则结果交错合并          |
| **JS 网页请求**  | `java.ajax()` / `webview` | 模拟 Legado 的 HTTP 请求 API |

### 规则语法示例

```
# CSS 选择器
@css:.book-name a@text

# XPath
@xpath://div[@class='book-name']/a/text()

# JSONPath
@json:$.data.list[*].name

# 正则替换
@css:.content@html##<[^>]+>##

# JavaScript 内嵌
@js:JSON.parse(result).data.name

# 多规则拼接
@css:.author@text && @css:.translator@text

# 交错合并
@css:.names@text %% @css:.urls@href
```

---

## 书源格式

书源是一个 JSON 文件，包含一个或多个书籍来源的定义。每个书源包含：

```json
{
  "bookSourceName": "示例书源",
  "bookSourceUrl": "https://www.example.com",
  "bookSourceGroup": "测试",
  "enabled": true,
  "searchUrl": "https://www.example.com/search?keyword={{key}}",
  "ruleSearch": {
    "bookList": "@css:ul.book-list li",
    "name": "a@text",
    "author": "span.author@text",
    "bookUrl": "a@href",
    "coverUrl": "img@src",
    "intro": "p.desc@text",
    "kind": "span.category@text",
    "lastChapter": "span.latest@text"
  },
  "ruleBookInfo": {
    "name": "@css:h1.book-name@text",
    "author": "@css:span.author@text",
    "coverUrl": "@css:.cover img@src",
    "intro": "@css:.intro@text",
    "kind": "@css:.category@text",
    "wordCount": "@css:.word-count@text",
    "lastChapter": "@css:.latest-chapter@text",
    "tocUrl": "@css:a.toc-link@href"
  },
  "ruleToc": {
    "chapterList": "@css:ul.chapter-list li",
    "chapterName": "a@text",
    "chapterUrl": "a@href",
    "nextTocUrl": "@css:a.next-page@href"
  },
  "ruleContent": {
    "content": "@css:div#content@html##<br/?>|&nbsp;|\n+##",
    "nextContentUrl": "@css:a.next-chapter@href",
    "title": "@css:h1.chapter-title@text",
    "replaceRegex": " |本章未完"
  },
  "header": "{\n\"User-Agent\": \"Mozilla/5.0...\"\n}"
}
```

详细规则说明请参见 [`doc/yue.md`](doc/yue.md)（书源规则）和 [`doc/ding.md`](doc/ding.md)（订阅源规则）。

---

## 项目结构

```
gopro/
├── reader.go             # 主入口：CLI 模式与 HTTP 模式切换
├── reader_test.go        # 集成测试
├── go.mod / go.sum       # Go 模块定义
│
├── server/               # Gin HTTP 服务
│   ├── server.go         # 服务初始化、SourceStore 书源管理、路由注册
│   └── handler.go        # 请求/响应结构体、API 处理函数
│
├── analyzer/             # 规则解析引擎
│   ├── analyzer.go       # 核心规则引擎：规则解析、求值
│   ├── source_rule.go    # 规则检测与拆分（CSS/XPath/JSON/Regex/JS）
│   ├── css.go            # CSS 选择器解析器
│   ├── xpath.go          # XPath 解析器
│   ├── jsonpath.go       # JSONPath 解析器
│   ├── regex.go          # 正则匹配与替换
│   └── legado_stub.go    # Legado JS 运行时桩（java.ajax, webview 等）
│
├── analyzeurl/           # URL 模板展开（{{key}}、{{page}} 等）
│
├── model/                # 数据类型
│   ├── booksource.go     # BookSource 及搜索/详情/目录/正文/发现规则结构体
│   ├── book.go           # Book / SearchBook
│   ├── chapter.go        # BookChapter
│   └── loader.go         # JSON 文件加载
│
├── webbook/              # 业务流程
│   ├── search.go         # 多书源并发搜索
│   ├── bookinfo.go       # 获取书籍详情
│   ├── chapterlist.go    # 获取目录列表
│   └── content.go        # 获取章节正文
│
├── rule/                 # 规则定义辅助
│
├── doc/                  # 文档与示例
│   ├── example.json      # 示例书源 JSON
│   ├── yue.md            # Legado 书源规则完整说明
│   └── ding.md           # Legado 订阅源规则说明
│
└── tool/                 # 调试脚本（非生产代码）
```

---

## 构建与测试

```bash
# 编译
go build ./...

# 运行测试
go test -v -run TestLoadSources -timeout 30s

# 静态分析
go vet ./...

# 格式化代码
gofmt -w .
```

---

## 技术栈

- **Go 1.25+**
- **Web 框架**: [Gin](https://github.com/gin-gonic/gin)
- **CSS 选择器**: [goquery](https://github.com/PuerkitoBio/goquery)（基于 jQuery 语法）
- **XPath**: [htmlquery](https://github.com/antchfx/htmlquery)
- **JSONPath**: [ojg](https://github.com/ohler55/ojg)
- **JavaScript 引擎**: [QuickJS (wazero)](https://github.com/fastschema/qjs) — 基于 WebAssembly 的沙箱 JS 引擎
- **HTTP 客户端**: `net/http` (标准库)

---

## 设计文档

- [`doc/yue.md`](doc/yue.md) — Legado 书源规则完整说明，涵盖搜索、发现、详情、目录、正文的规则语法与示例
- [`doc/ding.md`](doc/ding.md) — Legado 订阅源（RSS）规则说明与解析流程
- [`doc/example.json`](doc/example.json) — 可直接加载的真实书源 JSON 示例

---

## 许可证

本项目基于项目所在仓库的许可证发布。
