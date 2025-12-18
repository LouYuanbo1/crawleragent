package chrome

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
)

/*
type Crawlable interface {
	*entity.RowBossJobData
}
*/

// ChromeCrawler Chrome爬取器,用于获取指定URL的页面内容,并根据处理函数解析为指定类型的文档
type ChromeCrawler interface {
	PageContext() context.Context
	InitAndNavigate(url string) error
	PerformClick(selector string, clickCount, standardSleepSeconds, randomDelaySeconds int) error
	PerformScrolling(scrollTimes, standardSleepSeconds, randomDelaySeconds int) error
	SetNetworkListener(urlPattern string, respChan chan []types.NetworkResponse)
	Close()
}
