package service

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/service/chrome/param"
)

type ChromeService[C entity.Crawlable[D], D model.Document] interface {
	SetupNetworkListener(urlPattern string, toCrawlable func(body []byte) ([]C, error))
	ScrollCrawl(ctx context.Context, params *param.Scroll, batchSizeEmbedding int, toCrawlable func(body []byte) ([]C, error)) error
}
