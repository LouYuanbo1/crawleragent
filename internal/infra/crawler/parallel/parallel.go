package parallel

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/param"
)

type ParallelCrawler interface {
	Close()
	PerformAllUrlOperations(ctx context.Context, options []*param.UrlOperation) error
}
