package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/parallel"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	service "github.com/LouYuanbo1/crawleragent/internal/service/parallel"
	"github.com/LouYuanbo1/crawleragent/param"
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
	urlBoss           = "https://www.zhipin.com/web/geek/jobs?city=100010000&salary=406&experience=102&query=golang"
	urlPatternBoss    = "https://www.zhipin.com/wapi/zpgeek/search/joblist.json*"
	urlCnBlogs        = "https://www.cnblogs.com/"
	urlPatternCnBlogs = "https://www.cnblogs.com/AggSite/AggSitePostList*"
	selectorCnBlogs   = `//a[starts-with(@href, "/sitehome/p/") and text()=">"]`
	//urlBili           = "https://www.bilibili.com/"
	//urlPatternBili    = "https://api.bilibili.com/x/web-interface/wbi/index/top/feed/rcmd*"
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
	esJobClient, err := es.InitTypedEsClient(appcfg, 3)
	if err != nil {
		log.Fatalf("初始化Elasticsearch客户端失败: %v", err)
	}
	//创建索引并设置映射
	esJobClient.CreateIndexWithMapping(ctx)

	//初始化Rod爬虫
	/*
		parallelCrawler, err := parallel.InitRodPagePoolCrawler(appcfg, 3)
		if err != nil {
			log.Fatalf("初始化RodCrawler失败: %v", err)
		}
	*/
	parallelCrawler, err := parallel.InitRodBrowserPoolCrawler(appcfg, 3)
	if err != nil {
		log.Fatalf("初始化RodCrawler失败: %v", err)
	}

	//defer parallelCrawler.Close()

	//初始化Embedding模型
	embedder, err := embedding.InitEmbedder(ctx, appcfg, 1)
	if err != nil {
		log.Fatalf("初始化Embedder失败: %v", err)
	}

	//初始化爬虫服务
	//这里的crawler.InitCrawlerService函数用于初始化爬虫服务,将滚动爬虫、Elasticsearch客户端和Embedding模型组合起来
	serviceParallel := service.InitRodParallelService(parallelCrawler, esJobClient, embedder)

	respChanBoss := make(chan []types.NetworkResponse, 100)
	respChanCnblogs := make(chan []types.NetworkResponse, 100)
	//respChanBili := make(chan []types.NetworkResponse, 100)

	//创建监听器
	listenerBoss := &param.ListenerConfig{
		UrlPattern: urlPatternBoss,
		RespChan:   respChanBoss,
	}
	listenerCnblogs := &param.ListenerConfig{
		UrlPattern: urlPatternCnBlogs,
		RespChan:   respChanCnblogs,
	}

	/*
		listenerBili := &param.ListenerConfig{
			UrlPattern: urlPatternBili,
			RespChan:   respChanBili,
		}
	*/

	params := []*param.UrlOperation{
		{
			Url:           urlBoss,
			OperationType: param.OperationScroll,
			//每轮滚动爬取的次数
			//这里设置为5,表示每轮滚动爬取5次,你可以根据需要调整
			NumActions: 5,
			//标准 sleep 时间(秒)
			//这里设置为1秒,表示每次滚动爬取后,基础等待时间为1秒
			StandardSleepSeconds: 1,
			//随机延迟时间(秒)
			//这里设置为2秒,表示每次滚动爬取后,随机等待时间为0-2秒
			RandomDelaySeconds: 1,
			//实际等待实际为: StandardSleepSeconds + RandomDelaySeconds
			Listener: listenerBoss,
		},
		{
			Url:           urlCnBlogs,
			OperationType: param.OperationXClick,
			Selector:      selectorCnBlogs,
			//点击次数
			NumActions: 5,
			//标准 sleep 时间(秒)
			StandardSleepSeconds: 1,
			//随机延迟时间(秒)
			RandomDelaySeconds: 1,
			//实际等待实际为: StandardSleepSeconds + RandomDelaySeconds
			//监听的url
			Listener: listenerCnblogs,
		},
		/*
			{
				Url:           urlBili,
				OperationType: param.OperationScroll,
				//每轮滚动爬取的次数
				//这里设置为5,表示每轮滚动爬取5次,你可以根据需要调整
				NumActions: 5,
				//标准 sleep 时间(秒)
				//这里设置为1秒,表示每次滚动爬取后,基础等待时间为1秒
				StandardSleepSeconds: 1,
				//随机延迟时间(秒)
				//这里设置为2秒,表示每次滚动爬取后,随机等待时间为0-2秒
				RandomDelaySeconds: 1,
				//实际等待实际为: StandardSleepSeconds + RandomDelaySeconds
				Listener: listenerBili,
			},
		*/
	}
	//开始滚动爬取
	serviceParallel.ProcessRespChanWithIndexDocs(ctx, listenerBoss, func(body []byte) ([]*entity.RowBossJobData, error) {
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

	serviceParallel.ProcessRespChan(ctx, listenerCnblogs)

	//serviceParallel.ProcessRespChan(ctx, listenerBili)

	err = serviceParallel.PerformAllUrlOperations(ctx, params)
	if err != nil {
		log.Fatalf("滚动策略失败: %v", err)
	}

	parallelCrawler.Close()

	count, err := esJobClient.CountDocs(ctx)
	if err != nil {
		log.Fatalf("查询索引文档数量失败: %v", err)
	}
	//打印索引中的文档数量
	fmt.Printf("索引中的文档数量: %d\n", count)

}
