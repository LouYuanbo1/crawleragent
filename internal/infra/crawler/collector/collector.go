package collector

import (
	"github.com/LouYuanbo1/crawleragent/internal/service/crawler/option"
	"github.com/gocolly/colly/v2"
)

type CollyCrawler interface {
	Visit(url string) error
	Wait()
	OnRequest(options option.CollyCrawler, callback func(r *colly.Request))
	OnResponse(callback func(r *colly.Response))
	OnHTML(selector string, callback func(e *colly.HTMLElement))
	OnScraped(callback func(r *colly.Response))
	OnError(callback func(r *colly.Response, err error))
}
