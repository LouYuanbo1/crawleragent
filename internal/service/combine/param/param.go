package param

import (
	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
)

type DefaultStrategy[C entity.Crawlable[D], D model.Document] struct {
	EnableJavascript bool
	Selector         string
	HTMLFunc         func(e *colly.HTMLElement) error
	ToCrawlable      func(body []byte) ([]C, error)
}

type CustomStrategy[C entity.Crawlable[D], D model.Document] struct {
	EnableJavascript bool
	ActionsFunc      []chromedp.Action
	Selector         string
	HTMLFunc         func(e *colly.HTMLElement) error
	ToCrawlable      func(body []byte) ([]C, error)
}
