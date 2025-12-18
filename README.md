# crawleragent
一个结合了爬虫和AI智能体的项目，通过爬取网站扩充本地知识库并回答问题。

## 项目简介
CrawlerAgent是一个爬虫与AI智能体结合的工具，它能够：
1. 使用Colly,Chromedp,Rod爬取网站数据
2. 将爬取的数据存储到Elasticsearch中
3. 使用Ollama进行文本嵌入和大语言模型交互
4. 通过智能代理服务回答用户的问题

## 技术栈
1. 编程语言: Go 1.25.4
2. 爬虫框架:
    1. Colly - 轻量级、快速的爬虫框架
    2. Chromedp - 基于Chrome DevTools协议的爬虫框架(考虑暂缓更新,将来可能会替换为Rod爬虫)
    3. Rod - 基于Chrome DevTools协议的爬虫框架(与Chromedp相比,支持安全并发,Api更现代)
3. 数据存储: Elasticsearch 9.2.1
4. AI 模型:
    1. Eino - 工作流编排框架
    2. Ollama - 本地大语言模型(如llama,qwen等)
    3. Nomic-embed-text - 文本嵌入模型

## 项目结构
```plaintext
crawleragent/
├── cmd/                 # 命令行入口
│   ├── agent/           # AI智能体入口
│   ├── chromedp/        # Chromedp爬虫入口
│   ├── colly/           # Colly爬虫入口
|   └── rod/             # Rod爬虫入口
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
|       ├── chromedp/    # Chromedp爬虫服务
|       ├── colly/       # Colly爬虫服务
│       └── combine/     # 混合爬虫服务
├── go.mod               # Go模块定义
└── go.sum               # 依赖校验和
```

## 功能模块
1. 爬虫模块
    - Colly爬虫:轻量级、快速的HTTP爬虫
    1. 支持并发爬取
    2. 可配置的延迟和随机延迟
    3. 支持Cookie管理
    4. 支持HTML选择器(jquery风格,基于goquery实现)

    - Chromedp爬虫:基于Chrome DevTools协议(考虑暂缓更新,将来可能会替换为Rod爬虫)
    1. 支持浏览器中运行js代码
    2. 模拟浏览器操作(本项目中为滚动加载)
    3. 可配置用户数据目录
    4. 支持无头模式

    - Rod爬虫:基于Chrome DevTools协议
    1. 支持浏览器中运行js代码
    2. 模拟浏览器操作(本项目中为滚动加载)
    3. 可配置用户数据目录
    4. 支持无头模式
    5. 更现代的Api设计,支持并发操作

2. 数据存储模块
    - 使用Elasticsearch存储爬取的数据
    1. 支持自动创建索引和映射
    2. 支持批量索引
    3. 支持向量搜索

3. 嵌入模型模块
    - 使用Ollama的nomic-embed-text模型
    1. 支持批量嵌入
    2. 支持文本向量化

4. 智能代理模块
    - 基于Eino工作流编排框架
    1. 支持两种模式：
        - 搜索模式：使用Elasticsearch知识库回答问题
        - 聊天模式：直接使用LLM回答问题

    2. 支持流式输出

## 快速开始
### 安装依赖
1.  **克隆代码**
    ```bash
    git clone https://github.com/Louyuanbo1/crawleragent.git
    cd crawleragent
    ```
2.  **下载所有依赖**
    ```bash
    go mod download
    ```
    *（这一步会读取项目中的 `go.mod` 和 `go.sum`，将所有依赖下载到本地模块缓存。后续的 `go build` 或 `go run` 命令也会自动触发此操作。）*
    
### 配置文件
在每个命令行入口目录下都有一个appconfig文件夹，包含appconfig.json配置文件。你可以根据需要修改配置：
```json
appconfig.example.json
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
#### Rod爬虫
```bash
cd cmd/rod
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
//使用Rod爬虫爬取Boss直聘岗位示例:
var (
	url        = "https://www.zhipin.com/web/geek/jobs?city=100010000&salary=406&experience=102&query=golang"
	urlPattern = "*/joblist.json*"
)

func main() {
	appcfg, err := config.ParseConfig(appConfig)
	if err != nil {
		log.Fatalf("解析配置失败: %v", err)
	}

	fmt.Printf("Chromedp UserDataDir: %s\n", appcfg.Chromedp.UserDataDir)

	//context.Background()
	// 这是最常用的根Context，通常用在main函数、初始化或测试中，作为整个Context树的顶层。
	// 当你不知道使用哪个Context，或者没有可用的Context时，可以使用它作为起点。
	// 它永远不会被取消，没有超时时间，也没有值。
	ctx := context.Background()
	//运行前确保es服务启动完成
	//初始化Elasticsearch客户端
	esJobClient, err := es.InitTypedEsClient(appcfg)
	if err != nil {
		log.Fatalf("初始化Elasticsearch客户端失败: %v", err)
	}
	//创建索引并设置映射
	esJobClient.CreateIndexWithMapping(ctx)

	//初始化滚动爬虫
	//这里的handler func(body []byte) ([]*entity.RowBossJobData, error)
	//函数是滚动爬虫的回调函数,用于解析Boss直聘的岗位数据api返回的json数据
	//将json数据转换为泛型类型(此处为entity.RowBossJobData)的切片,并返回

	scrollCrawler, err := chrome.InitRodCrawler(appcfg)
	if err != nil {
		log.Fatalf("初始化RodCrawler失败: %v", err)
	}
	defer scrollCrawler.Close()

	//初始化Embedding模型
	embedder, err := embedding.InitEmbedder(ctx, appcfg)
	if err != nil {
		log.Fatalf("初始化Embedder失败: %v", err)
	}

	//初始化爬虫服务
	//这里的crawler.InitCrawlerService函数用于初始化爬虫服务,将滚动爬虫、Elasticsearch客户端和Embedding模型组合起来
	//爬虫服务负责协调滚动爬虫的运行,将爬取到的数据转换为文档,并使用Embedding模型生成向量表示,最后将文档和向量索引到Elasticsearch中
	service := service.InitChromedpService(scrollCrawler, esJobClient, embedder)
	service.SetNetworkListener(ctx, urlPattern, 100, func(body []byte) ([]*entity.RowBossJobData, error) {
		var jsonData struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			ZpData  struct {
				HasMore    bool                    `json:"hasMore"`
				JobResList []entity.RowBossJobData `json:"jobList"`
			} `json:"zpData"`
		}

		if err := json.Unmarshal(body, &jsonData); err != nil {
			return nil, fmt.Errorf("JSON解析失败: %v", err)
		}

		if jsonData.Code != 0 {
			return nil, fmt.Errorf("API返回错误: %d - %s", jsonData.Code, jsonData.Message)
		}

		results := make([]*entity.RowBossJobData, 0, len(jsonData.ZpData.JobResList))
		for _, job := range jsonData.ZpData.JobResList {
			results = append(results, &entity.RowBossJobData{
				EncryptJobId:     job.EncryptJobId,
				SecurityId:       job.SecurityId,
				JobName:          job.JobName,
				SalaryDesc:       job.SalaryDesc,
				BrandName:        job.BrandName,
				BrandScaleName:   job.BrandScaleName,
				CityName:         job.CityName,
				AreaDistrict:     job.AreaDistrict,
				BusinessDistrict: job.BusinessDistrict,
				JobLabels:        job.JobLabels,
				Skills:           job.Skills,
				JobExperience:    job.JobExperience,
				JobDegree:        job.JobDegree,
				WelfareList:      job.WelfareList,
			})
		}
		return results, nil
	},
	)
	scrollParams := &param.Scroll{
		Url: url,
		//滚动爬虫监听的api
		//滚动爬虫运行的轮数,分轮爬行对内存更友好,可以将2*5 改成 5*2,根据实际情况调整
		//这里设置为1,表示只运行一轮,你可以根据需要调整
		Rounds: 2,
		//每轮滚动爬取的次数
		//这里设置为5,表示每轮滚动爬取5次,你可以根据需要调整
		ScrollTimes: 5,
		//标准 sleep 时间(秒)
		//这里设置为1秒,表示每次滚动爬取后,基础等待时间为1秒
		StandardSleepSeconds: 1,
		//随机延迟时间(秒)
		//这里设置为2秒,表示每次滚动爬取后,随机等待时间为0-2秒
		RandomDelaySeconds: 2,
		//实际等待实际为: StandardSleepSeconds + RandomDelaySeconds
	}
	//这里设置为5,表示每次同时词嵌入5个文档,你可以根据需要调整
	err = service.ScrollCrawl(ctx, scrollParams)
	if err != nil {
		log.Fatalf("滚动爬取失败: %v", err)
	}
	count, err := esJobClient.CountDocs(ctx)
	//打印索引中的文档数量
	fmt.Printf("索引中的文档数量: %d\n", count)

	if err != nil {
		log.Fatalf("滚动爬取失败: %v", err)
	}
}
```

### 智能代理示例
使用前记得先运行爬虫,将数据存储到Elasticsearch中,否则只能使用聊天模式
```bash
cd cmd/agent
go run main.go
```

```go
欢迎使用CrawlerAgent!
注意:当请求以'查询模式'或'搜索模式:'开头时,会使用Es知识库,否则会默认为聊天模式。
知识库内容越多,描述越完善,推荐结果越准确。
请输入您的请求:

// 搜索模式示例
搜索模式: 北京的Golang岗位

// 聊天模式示例
什么是Golang?

// 智能体会根据您的请求,从知识库中提取相关信息,并使用LLM生成响应
// 请求越详细清晰,爬取的数据越多,返回的结果越准确
// 目前搜索知识库的逻辑较为简单,仅提供词嵌入向量余弦相似度搜索
// 未来可以考虑添加更多的搜索策略
// 如果您有更好的方法或者建议,欢迎issue告诉我!
// 之后有时间会考虑添加更多的功能,如支持更多的搜索策略、添加更多的聊天模式等
```

## 项目扩展
### 添加新的爬虫
1. 在internal/domain/entity中定义新的实体
2. 在internal/domain/model中定义对应的文档模型
3. 在调用api时根据待爬网站特征,选择合适的爬虫api并手动设置转换函数


### 修改智能体工作流
1. 在internal/service/agent中添加新的节点
2. 在internal/service/agent/param中添加新的参数
3. 修改internal/service/agent/agent.go中的工作流

## 引用
1. [Eino](https://github.com/cloudwego/eino)
2. [Eino-Ext](https://github.com/cloudwego/eino-ext)
3. [Colly](https://github.com/gocolly/colly)
4. [Chromedp](https://github.com/chromedp/chromedp)
5. [Rod](https://github.com/go-rod/rod)
6. [Elasticsearch](https://github.com/elastic/go-elasticsearch)

## 引用文档
1. [Eino文档](https://www.cloudwego.io/zh/docs/eino/)
2. [Colly文档](https://pkg.go.dev/github.com/gocolly/colly)
3. [Chromedp文档](https://pkg.go.dev/github.com/chromedp/chromedp)
4. [Rod文档](https://pkg.go.dev/github.com/go-rod/rod)
5. [Elasticsearch文档](https://www.elastic.co/docs/reference/elasticsearch/clients/go)

## 未来计划
1. 增强智能体的搜索能力,寻找更强的搜索策略
2. 编写更复杂的graph,增加更多工具功能
3. 考虑添加更多的功能,如支持更多类型的爬虫方法等

## 写在最后
1. 使用本项目时,请遵守相关法律法规,增加延迟和减少对同一网站并发数,避免对目标网站造成过大负担。
2. 本项目仅作为学习和研究用途,不建议在生产环境中使用。
3. 本项目的大模型编排基于Cloudwego的Eino框架,如果您对Eino框架感兴趣,可以查看其[文档](https://www.cloudwego.io/zh/docs/eino/).希望Eino官方能够继续完善和维护这个项目,为Go语言大模型生态做出贡献(请将文档写的更纤细一些/(ㄒoㄒ)/~~)。
4. 个人工作征集中,莫斯科国立大学26届本科生,正在寻找Golang相关工作机会。如果您有需求请联系我(通过issue或者email): 1532584362@qq.com
