package service

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
	"github.com/LouYuanbo1/crawleragent/param"
)

type ParallelService[C entity.Crawlable[D], D model.Document] interface {
	PerformOpentionsALL(options []*param.URLOperation) error
	ProcessResponseChan(ctx context.Context, RespChan <-chan []types.NetworkResponse)
	ProcessResponseChanWithIndexDocs(ctx context.Context, RespChan <-chan []types.NetworkResponse, toCrawlable func(body []byte) ([]C, error))
}
