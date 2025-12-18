package service

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/param"
)

type ParallelService[C entity.Crawlable[D], D model.Document] interface {
	StartRouter()
	PerformOpentionsALL(options []*param.URLOperation) error
	SetNetworkListenerWithIndexDocs(ctx context.Context, urlPattern string, RespChanSize int, toCrawlable func(body []byte) ([]C, error))
	SetNetworkListener(ctx context.Context, urlPattern string, RespChanSize int)
}
