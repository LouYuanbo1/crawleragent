package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/chrome"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/collector"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"

	"github.com/LouYuanbo1/crawleragent/internal/service/combine/param"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
)

type CombineService[C entity.Crawlable[D], D model.Document] interface {
	ChromedpCrawler() chrome.ChromedpCrawler
	CollyCrawler() collector.CollyCrawler
	TypedEsClient() es.TypedEsClient[D]
	Embedder() embedding.Embedder
	Crawl(ctx context.Context, url string) error
	DefaultStrategy(params *param.DefaultStrategy[C, D]) error
	CustomStrategy(params *param.CustomStrategy[C, D]) error
	RecursiveCrawling(hrefSelector string)
	OnResponse(handler func(r *colly.Response) error)
	OnHTML(selector string, handler func(r *colly.HTMLElement) error)
	OnScraped(toCrawlable func(body []byte) ([]C, error))
}

type combineService[C entity.Crawlable[D], D model.Document] struct {
	chromedpCrawler chrome.ChromedpCrawler
	collyCrawler    collector.CollyCrawler
	typedEsClient   es.TypedEsClient[D]
	embedder        embedding.Embedder
	processSem      chan struct{}
	embedSem        chan struct{}
}

func InitCombineService[C entity.Crawlable[D], D model.Document](
	chromedpCrawler chrome.ChromedpCrawler,
	collyCrawler collector.CollyCrawler,
	typedEsClient es.TypedEsClient[D],
	embedder embedding.Embedder,
	processSemSize int,
	embedSemSize int,
) CombineService[C, D] {
	return &combineService[C, D]{
		chromedpCrawler: chromedpCrawler,
		collyCrawler:    collyCrawler,
		typedEsClient:   typedEsClient,
		embedder:        embedder,
		processSem:      make(chan struct{}, processSemSize),
		embedSem:        make(chan struct{}, embedSemSize),
	}
}

func (cs *combineService[C, D]) ChromedpCrawler() chrome.ChromedpCrawler {
	return cs.chromedpCrawler
}

func (cs *combineService[C, D]) CollyCrawler() collector.CollyCrawler {
	return cs.collyCrawler
}

func (cs *combineService[C, D]) TypedEsClient() es.TypedEsClient[D] {
	return cs.typedEsClient
}

func (cs *combineService[C, D]) Embedder() embedding.Embedder {
	return cs.embedder
}

func (cs *combineService[C, D]) Crawl(ctx context.Context, url string) error {
	log.Printf("Crawl, url: %s", url)
	err := cs.collyCrawler.Visit(url)
	if err != nil {
		return fmt.Errorf("visit error, url: %s, error: %w", url, err)
	}
	cs.collyCrawler.Wait()
	log.Printf("Crawl, url: %s, wait done", url)
	return nil
}

func (cs *combineService[C, D]) DefaultStrategy(params *param.DefaultStrategy[C, D]) error {
	if params.Selector == "" || params.HTMLFunc == nil {
		return fmt.Errorf("selector or HTMLFunc or ToCrawlable is empty")
	}
	if params.EnableJavascript {
		cs.OnResponse(func(r *colly.Response) error {
			err := cs.defaultCrawlingFromChrome(r)
			if err != nil {
				log.Printf("defaultCrawlingFromChrome error, url: %s, error: %s", r.Request.URL, err)
				return err
			}
			return nil
		})
	}
	cs.OnHTML(params.Selector, params.HTMLFunc)
	if params.OptToCrawlable != nil {
		cs.OnScraped(params.OptToCrawlable)
	}
	log.Printf("DefaultStrategy, selector: %s", params.Selector)
	return nil
}

func (cs *combineService[C, D]) RecursiveCrawling(hrefSelector string) {
	cs.collyCrawler.OnHTML(hrefSelector, func(el *colly.HTMLElement) {
		log.Println("visiting: ", el.Attr("href"))

		err := el.Request.Visit(el.Attr("href"))
		if err != nil {
			// Ignore already visited error, this appears too often
			var alreadyVisited *colly.AlreadyVisitedError
			if !errors.As(err, &alreadyVisited) {
				log.Printf("already visited: %s", err.Error())
			}
		}
	})
}

func (cs *combineService[C, D]) defaultCrawlingFromChrome(response *colly.Response) error {
	pageCtx := cs.chromedpCrawler.PageContext()
	var res string

	err := chromedp.Run(pageCtx,
		chromedp.Navigate(response.Request.URL.String()),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.InnerHTML("html", &res),
	)
	if err != nil {
		return fmt.Errorf("chromedp execution: %w", err)
	}

	response.Body = []byte(res)

	return nil
}

func (cs *combineService[C, D]) CustomStrategy(params *param.CustomStrategy[C, D]) error {
	if params.Selector == "" || params.HTMLFunc == nil {
		return fmt.Errorf("selector or HTMLFunc is empty")
	}
	if params.EnableJavascript {
		cs.OnResponse(func(r *colly.Response) error {
			err := cs.customCrawlingFromChrome(r, params.ActionsFunc)
			if err != nil {
				log.Printf("customCrawlingFromChrome error, url: %s, error: %s", r.Request.URL, err)
				return err
			}
			return nil
		})
	}
	cs.OnHTML(params.Selector, params.HTMLFunc)
	if params.OptToCrawlable != nil {
		cs.OnScraped(params.OptToCrawlable)
	}
	log.Printf("CustomStrategy, selector: %s", params.Selector)
	return nil
}

func (cs *combineService[C, D]) customCrawlingFromChrome(response *colly.Response, actionsFunc []chromedp.Action) error {
	pageCtx := cs.chromedpCrawler.PageContext()
	var res string

	err := chromedp.Run(pageCtx,
		actionsFunc...,
	)
	if err != nil {
		return fmt.Errorf("chromedp execution: %w", err)
	}

	response.Body = []byte(res)

	return nil
}

func (cs *combineService[C, D]) OnResponse(handler func(r *colly.Response) error) {
	//在colly的OnResponse回调中，只有在有响应时才会被调用。所以，信号量的获取尝试也只会在有响应时发生。
	cs.collyCrawler.OnResponse(func(r *colly.Response) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// 尝试向信号量通道发送一个空结构体
		select {
		case cs.processSem <- struct{}{}:
			// 如果发送成功，则注册一个函数在退出时从信号量通道接收（释放信号量）
			defer func() { <-cs.processSem }()
		case <-timeoutCtx.Done():
			// 等待5秒后超时
			cs.handleRateLimit(r.Request)
			return
		}
		err := handler(r)
		if err != nil {
			log.Printf("Handler error, url: %s, error: %s", r.Request.URL, err)
			return
		}
	})
}

func (cs *combineService[C, D]) OnHTML(selector string, handler func(r *colly.HTMLElement) error) {
	//在colly的OnResponse回调中，只有在有响应时才会被调用。所以，信号量的获取尝试也只会在有响应时发生。
	cs.collyCrawler.OnHTML(selector, func(r *colly.HTMLElement) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// 尝试向信号量通道发送一个空结构体
		select {
		case cs.processSem <- struct{}{}:
			// 如果发送成功，则注册一个函数在退出时从信号量通道接收（释放信号量）
			defer func() { <-cs.processSem }()
		case <-timeoutCtx.Done():
			// 等待5秒后超时
			cs.handleRateLimit(r.Request)
			return
		}
		err := handler(r)
		if err != nil {
			log.Printf("Handler error, url: %s, error: %s", r.Request.URL, err)
			return
		}
	})
}

func (cs *combineService[C, D]) OnScraped(toCrawlable func(body []byte) ([]C, error)) {
	//在colly的OnResponse回调中，只有在有响应时才会被调用。所以，信号量的获取尝试也只会在有响应时发生。
	cs.collyCrawler.OnScraped(func(r *colly.Response) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		select {
		// 尝试向信号量通道发送一个空结构体
		case cs.processSem <- struct{}{}:
			// 如果发送成功，则注册一个函数在退出时从信号量通道接收（释放信号量）
			defer func() { <-cs.processSem }()
		case <-timeoutCtx.Done():
			// 等待5秒后超时
			cs.handleRateLimit(r.Request)
			return
		}
		data, err := toCrawlable(r.Body)
		if err != nil {
			log.Printf("Handler error, url: %s, error: %s", r.Request.URL, err)
			return
		}
		if len(data) == 0 {
			return
		}
		docs := make([]D, 0, len(data))
		for _, d := range data {
			docs = append(docs, d.ToDocument())
		}
		// 嵌入文档
		cs.embeddingDocs(docs)
		cs.indexDocs(docs)
	})
}

func (cs *combineService[C, D]) embeddingDocs(docs []D) {
	// 获取信号量
	cs.embedSem <- struct{}{}
	defer func() { <-cs.embedSem }() // 释放信号量
	// 从配置中获取批量处理大小
	batchSizeEmbedding := cs.embedder.BatchSize()
	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	embeddingStrings := make([]string, 0, len(docs))
	for _, doc := range docs {
		embeddingStrings = append(embeddingStrings, doc.GetEmbeddingString())
	}
	var embeddingVectors [][]float32
	var err error
	if len(embeddingStrings) < batchSizeEmbedding {
		embeddingVectors, err = cs.embedder.Embed(reqCtx, embeddingStrings)
		if err != nil {
			log.Printf("Embed error: %v", err)
		}
		for i := range embeddingVectors {
			docs[i].SetEmbedding(embeddingVectors[i])
		}
	} else {
		for i := 0; i < len(embeddingStrings); i += batchSizeEmbedding {
			end := i + batchSizeEmbedding
			end = min(end, len(embeddingStrings))
			embeddingVectors, err = cs.embedder.Embed(reqCtx, embeddingStrings[i:end])
			if err != nil {
				log.Printf("Embed error: %v", err)
			}
			for j := range embeddingVectors {
				docs[i+j].SetEmbedding(embeddingVectors[j])
				log.Printf("Indexed doc %s with embedding len %d", docs[i+j].GetID(), len(docs[i+j].GetEmbedding()))
			}
		}
	}
}

func (cs *combineService[C, D]) indexDocs(docs []D) {

	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := cs.typedEsClient.BulkIndexDocsWithID(reqCtx, docs); err != nil {
		log.Printf("Bulk index error: %v", err)
		return
	}
}

func (cs *combineService[C, D]) handleRateLimit(r *colly.Request) {
	// 简单的丢弃策略，也可以实现排队或其他策略
	fmt.Printf("Rate limit hit, url: %s, discarding...\n", r.URL)
}
