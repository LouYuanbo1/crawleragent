package chrome

import (
	"context"
	"sync"
)

/*
type Crawlable interface {
	*entity.RowBossJobData
}
*/

// ChromedpCrawler Chromedp爬取器,用于获取指定URL的页面内容,并根据处理函数解析为指定类型的文档
type ChromedpCrawler interface {
	PageContext() context.Context
	RequestCache() *sync.Map
	InitAndNavigate(url string) error
	ResetAndScroll(scrollTimes, standardSleepSeconds, randomDelaySeconds int) error
	Close()
}
