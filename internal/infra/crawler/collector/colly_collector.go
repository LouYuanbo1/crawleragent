package collector

import (
	"fmt"
	"log"
	"net/http/cookiejar"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/collector/option"
	"github.com/gocolly/colly/v2"
)

type collyCrawler struct {
	colly *colly.Collector
}

func InitCollyCrawler(config *config.Config) CollyCrawler {
	var opts []colly.CollectorOption
	opts = append(opts,
		colly.MaxDepth(config.Colly.MaxDepth),
		colly.Async(config.Colly.Async),
		colly.UserAgent(config.Colly.UserAgent),
		colly.AllowedDomains(config.Colly.AllowedDomains...),
	)
	if config.Colly.IgnoreRobotsTxt {
		opts = append(opts, colly.IgnoreRobotsTxt())
	}
	c := colly.NewCollector(opts...)
	c.Limit(&colly.LimitRule{
		Delay:       time.Duration(config.Colly.Delay) * time.Second,
		RandomDelay: time.Duration(config.Colly.RandomDelay) * time.Second,
	})
	if config.Colly.EnableCookieJar {
		jar, err := cookiejar.New(config.Colly.CookieJarOptions)
		if err != nil {
			panic(err)
		}
		c.SetCookieJar(jar)
	}
	log.Printf("InitCollyCrawler, maxDepth: %d, async: %v, delay: %d, randomDelay: %d", config.Colly.MaxDepth, config.Colly.Async, config.Colly.Delay, config.Colly.RandomDelay)
	return &collyCrawler{
		colly: c,
	}
}

func (c *collyCrawler) Visit(url string) error {
	err := c.colly.Visit(url)
	if err != nil {
		return fmt.Errorf("访问URL失败: %w", err)
	}
	return nil
}

func (c *collyCrawler) Wait() {
	c.colly.Wait()
}

func (c *collyCrawler) OnRequest(options option.CollyRequest, callback func(r *colly.Request)) {
	c.colly.OnRequest(func(r *colly.Request) {
		if options.UserAgent != "" {
			r.Headers.Set("User-Agent", options.UserAgent)
		}
		if options.Headers != nil {
			for k, v := range options.Headers {
				r.Headers.Set(k, v)
			}
		}
		callback(r)
	})
}

func (c *collyCrawler) OnResponse(callback func(r *colly.Response)) {
	c.colly.OnResponse(callback)
}

func (c *collyCrawler) OnHTML(selector string, callback func(e *colly.HTMLElement)) {
	c.colly.OnHTML(selector, callback)
}

func (c *collyCrawler) OnScraped(callback func(r *colly.Response)) {
	c.colly.OnScraped(callback)
}

func (c *collyCrawler) OnError(callback func(r *colly.Response, err error)) {
	c.colly.OnError(callback)
}
