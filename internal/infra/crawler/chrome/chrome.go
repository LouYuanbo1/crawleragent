package chrome

import "context"

/*
type Crawlable interface {
	*entity.RowBossJobData
}
*/

// ChromeCrawler Chrome爬取器,用于获取指定URL的页面内容,并根据处理函数解析为指定类型的文档
type ChromeCrawler interface {
	PageContext() context.Context
	InitAndNavigate(url string) error
	PerformScrolling(scrollTimes, standardSleepSeconds, randomDelaySeconds int) error
	Close()
}
