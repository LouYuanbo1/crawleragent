package parallel

import (
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
	"github.com/LouYuanbo1/crawleragent/param"
)

type ParallelCrawler interface {
	StartRouter()
	Close()
	PerformOpentionsALL(options []*param.URLOperation) error
	SetNetworkListener(urlPattern string, respChan chan []types.NetworkResponse)
}
