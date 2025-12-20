package parallel

import (
	"github.com/LouYuanbo1/crawleragent/param"
)

type ParallelCrawler interface {
	Close()
	PerformAllListnerOperations(options []*param.ListenerOperation) error
}
