package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/collector"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	service "github.com/LouYuanbo1/crawleragent/internal/service/colly"

	"github.com/gocolly/colly/v2"
)

//go:embed appconfig/appconfig.json
var appConfig []byte

func main() {
	appcfg, err := config.ParseConfig(appConfig)
	if err != nil {
		log.Fatalf("解析配置失败: %v", err)
	}

	println(appcfg.Colly.UserAgent)

	ctx := context.Background()
	collyCollector := collector.InitCollyCrawler(appcfg)
	esJobClient, err := es.InitTypedEsClient(appcfg)
	if err != nil {
		log.Fatalf("初始化Elasticsearch客户端失败: %v", err)
	}
	embedder, err := embedding.InitEmbedder(ctx, appcfg)
	if err != nil {
		log.Fatalf("初始化Embedder失败: %v", err)
	}
	service := service.InitCollyService(collyCollector, esJobClient, embedder, 8, 1)
	service.CollyCrawler().OnHTML("body", func(e *colly.HTMLElement) {
		fmt.Println(e.Text[:200])
	})
	service.RecursiveCrawling("a[href*=https://www.bilibili.com/video/]")

	if err := service.Visit("https://www.bilibili.com"); err != nil {
		log.Fatalf("访问URL失败: %v", err)
	}

	service.Wait()
}
