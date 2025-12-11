package param

import (
	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
)

type CombineDefaultStrategy[C entity.Crawlable[D], D model.Document] struct {
	EnableJavascript bool
	Selector         string
	HTMLFunc         func(e *colly.HTMLElement) error
	ToCrawlable      func(body []byte) ([]C, error)
}

type CombineCustomStrategy[C entity.Crawlable[D], D model.Document] struct {
	EnableJavascript bool
	ActionsFunc      []chromedp.Action
	Selector         string
	HTMLFunc         func(e *colly.HTMLElement) error
	ToCrawlable      func(body []byte) ([]C, error)
}

// ChromeScroll 滚动爬取选项,用于配置滚动爬取的行为
type ChromeScroll struct {
	Url                  string `json:"url"`
	UrlPattern           string `json:"url_pattern"`
	Rounds               int    `json:"rounds"`
	ScrollTimes          int    `json:"scroll_times"`
	StandardSleepSeconds int    `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int    `json:"random_delay_seconds"`
}
