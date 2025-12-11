package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/chrome"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/collector"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	"github.com/LouYuanbo1/crawleragent/internal/service/crawler"
	"github.com/LouYuanbo1/crawleragent/internal/service/crawler/param"
	"github.com/gocolly/colly/v2"
)

//使用go:embed嵌入appconfig.json文件
//下方注释重要,不能删除
//在实际使用时，注意与文件名的对应，Github上保存的appconfig_example.json文件为样例，以实际为准,比如我这里是appconfig.json
//When using it in practice, pay attention to the correspondence between the filename and the actual filename.
//The appconfig_example.json file saved on GitHub is just an example;
//use your own file, for example, mine is appconfig.json.

//go:embed appconfig/appconfig.json
var appConfig []byte

// 定义要爬取的URL（Boss直聘作为参考）
// 这里的URL是Boss直聘的Golang岗位搜索结果页，你可以根据需要修改
// urlPattern是Boss直聘的岗位数据api中的一部分,你可以通过f12寻找到它
var (
	//url = "https://www.zhipin.com/web/geek/jobs?city=100010000&salary=405&experience=102&query=golang"
	url = "https://www.bilibili.com"
	//urlPattern = "joblist.json"
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
	collector := collector.InitCollyCrawler(appcfg)

	scrollCrawler := chrome.InitChromedpCrawler(ctx, appcfg)
	defer scrollCrawler.Close()

	//初始化Embedding模型
	embedder, err := embedding.InitEmbedder(ctx, appcfg)
	if err != nil {
		log.Fatalf("初始化Embedder失败: %v", err)
	}

	service := crawler.InitCombineService(scrollCrawler, collector, esJobClient, embedder, 10, 1)
	/*
		params := &param.CombineCrawlerDefaultStrategy[*entity.RowBossJobData, *model.BossJobDoc]{
			EnableJavascript: false,
			Selector:         "head title",
			HTMLFunc: func(e *colly.HTMLElement) error {
				fmt.Println(e.Text)
				return nil
			},
		}
		service.DefaultStrategy(params)
	*/
	params := &param.CombineCrawlerCustomStrategy[*entity.RowBossJobData, *model.BossJobDoc]{
		EnableJavascript: false,
		Selector:         "head title",
		HTMLFunc: func(e *colly.HTMLElement) error {
			fmt.Println(e.Text)
			return nil
		},
	}
	service.CustomStrategy(params)

	service.Crawl(ctx, url)
}
