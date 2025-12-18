package service

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/service/chrome/param"
)

type ChromeService[C entity.Crawlable[D], D model.Document] interface {
	SetNetworkListenerWithIndexDocs(ctx context.Context, urlPattern string, RespChanSize int, toCrawlable func(body []byte) ([]C, error))
	SetNetworkListener(ctx context.Context, urlPattern string, RespChanSize int)
	ScrollStrategy(ctx context.Context, params *param.Scroll) error
	ClickStrategy(ctx context.Context, params *param.Click) error
}
