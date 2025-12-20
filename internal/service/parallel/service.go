package service

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/param"
)

type ParallelService[C entity.Crawlable[D], D model.Document] interface {
	PerformAllListnerOperations(options []*param.ListenerOperation) error
	ProcessRespChan(ctx context.Context, listener *param.ListenerConfig)
	ProcessRespChanWithIndexDocs(ctx context.Context, listener *param.ListenerConfig, toCrawlable func(body []byte) ([]C, error))
}
