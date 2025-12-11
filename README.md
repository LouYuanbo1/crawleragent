# crawleragent
一个结合了爬虫和AI智能体的项目，通过爬取网站扩充本地知识库并回答问题。

## 项目简介

CrawlerAgent是一个爬虫与AI智能体结合的工具，它能够：

使用Colly,Chromedp爬取网站数据并支持两种混合爬虫

将爬取的数据存储到Elasticsearch中

使用Ollama进行文本嵌入和大语言模型交互

通过智能代理服务回答用户的问题

## 技术栈

编程语言: Go 1.25.4

爬虫框架:

Colly - 轻量级、快速的爬虫框架

Chromedp - 基于Chrome DevTools协议的爬虫框架

数据存储: Elasticsearch 9.2.1

AI 模型:

Eino - 工作流编排框架

Ollama - 本地大语言模型

Nomic-embed-text - 文本嵌入模型

## 项目结构

```plaintext
crawleragent/
├── cmd/                  # 命令行入口
│   ├── agent/           # AI智能体入口
│   ├── chromedp/        # Chromedp爬虫入口
│   ├── colly/           # Colly爬虫入口
│   └── combine/         # 混合爬虫入口
├── internal/            # 内部包
│   ├── config/          # 配置管理
│   ├── domain/          # 领域模型
│   │   ├── entity/      # 实体定义
│   │   └── model/       # 数据模型
│   ├── infra/           # 基础设施
│   │   ├── crawler/     # 爬虫实现
│   │   ├── embedding/   # 嵌入模型实现
│   │   ├── llm/         # LLM实现
│   │   └── persistence/ # 持久化实现
│   └── service/         # 业务服务
│       ├── agent/       # 智能体服务
│       └── crawler/     # 爬虫服务
├── go.mod               # Go模块定义
└── go.sum               # 依赖校验和
```

## 功能模块

1. 爬虫模块
    Colly爬虫
    轻量级、快速的HTTP爬虫

    支持并发爬取

    可配置的延迟和随机延迟

    支持Cookie管理

    支持HTML选择器

    Chromedp爬虫
    基于Chrome DevTools协议

    支持JavaScript渲染的页面

    支持滚动加载

    可配置用户数据目录

    支持无头模式

2. 数据存储模块
    使用Elasticsearch存储爬取的数据

    支持自动创建索引和映射

    支持批量索引

    支持向量搜索

3. 嵌入模型模块
    使用Ollama的nomic-embed-text模型

    支持批量嵌入

    支持文本向量化

4. 智能代理模块
    基于Eino工作流编排框架

    支持意图检测

    支持两种模式：

    搜索模式：使用Elasticsearch知识库回答问题

    聊天模式：直接使用LLM回答问题

    支持流式输出

## 快速开始

### 安装依赖

```bash
go mod download
```

### 配置文件

在每个命令行入口目录下都有一个appconfig文件夹，包含appconfig.json配置文件。你可以根据需要修改配置：
```json
{
  "elasticsearch": {
    "username": "elastic",
    "password": "password",
    "address": "http://localhost:9200"
  },
  "chromedp": {
    "user_data_dir": "user_data_dir",
    "headless": true,
    "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
  },
  "colly": {
    "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    "allowed_domains": ["www.bilibili.com"],
    "max_depth": 1
  },
  "embedder": {
    "host": "http://localhost",
    "port": 11434,
    "model": "nomic-embed-text",
    "batch_size": 5
  },
  "llm": {
    "host": "http://localhost",
    "port": 11434,
    "model": "llama3"
  }
}
```

### 运行爬虫
#### Colly爬虫
```bash
cd cmd/colly
go run main.go
```
#### Chromedp爬虫
```bash
cd cmd/chromedp
go run main.go
```
#### 运行智能体
```bash
cd cmd/agent
go run main.go
```

## 使用示例

### 爬取示例
```go
Colly爬虫爬取B站示例：

go
// 初始化Colly服务
service := crawler.InitCollyService(collyCollector, esJobClient, embedder, 8, 1)

// 设置HTML处理函数
service.CollyCrawler().OnHTML("head title", func(e *colly.HTMLElement) {
    fmt.Println(e.Text)
})

// 访问B站
if err := service.Visit("https://www.bilibili.com"); err != nil {
    log.Fatalf("访问URL失败: %v", err)
}
// 等待所有请求完成
service.Wait()
```

### 智能代理示例
```bash
cd cmd/agent
go run main.go

```go
欢迎使用CrawlerAgent!
注意:当请求以'查询模式'或'搜索模式:'开头时,会使用Es知识库,否则会默认为聊天模式。
知识库内容越多,描述越完善,推荐结果越准确。
请输入您的请求:

// 搜索模式示例
搜索模式: 北京的Golang岗位

// 聊天模式示例
什么是Golang?

```

## 项目扩展
### 添加新的爬虫
在internal/domain/entity中定义新的实体

在internal/domain/model中定义对应的文档模型

在internal/service/crawler中实现新的爬虫服务

在cmd目录下创建新的命令行入口

### 添加新的智能代理功能
在internal/service/agent中添加新的节点

在internal/service/agent/param中添加新的参数

### 修改智能体工作流
修改internal/service/agent/agent.go中的工作流