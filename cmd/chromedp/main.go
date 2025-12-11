package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/chrome"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"

	service "github.com/LouYuanbo1/crawleragent/internal/service/chromedp"
	"github.com/LouYuanbo1/crawleragent/internal/service/chromedp/param"
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
	url        = "https://www.zhipin.com/web/geek/jobs?city=100010000&salary=405&experience=102&query=golang"
	urlPattern = "joblist.json"
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

	scrollCrawler := chrome.InitChromedpCrawler(ctx, appcfg)
	defer scrollCrawler.Close()

	//初始化Embedding模型
	embedder, err := embedding.InitEmbedder(ctx, appcfg)
	if err != nil {
		log.Fatalf("初始化Embedder失败: %v", err)
	}

	//初始化爬虫服务
	//这里的crawler.InitCrawlerService函数用于初始化爬虫服务,将滚动爬虫、Elasticsearch客户端和Embedding模型组合起来
	//爬虫服务负责协调滚动爬虫的运行,将爬取到的数据转换为文档,并使用Embedding模型生成向量表示,最后将文档和向量索引到Elasticsearch中
	service := service.InitChromedpService(scrollCrawler, esJobClient, embedder, 5, 1)
	scrollParams := &param.Scroll{
		Url: url,
		//滚动爬虫监听的api
		UrlPattern: urlPattern,
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
	err = service.ScrollCrawl(ctx, scrollParams, 5, func(body []byte) ([]*entity.RowBossJobData, error) {
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
	if err != nil {
		log.Fatalf("滚动爬取失败: %v", err)
	}
}
