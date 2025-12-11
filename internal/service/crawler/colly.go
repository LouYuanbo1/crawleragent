package crawler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/collector"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	"github.com/gocolly/colly/v2"
)

type CollyService[C entity.Crawlable[D], D model.Document] interface {
	CollyCrawler() collector.CollyCrawler
	TypedEsClient() es.TypedEsClient[D]
	Embedder() embedding.Embedder
	Visit(url string) error
	Wait()
	HandleResponse(ctx context.Context, toCrawlable func(body []byte) ([]C, error))
	HandleHTML(ctx context.Context, selector string, toCrawlable func(r *colly.HTMLElement) ([]C, error))
}

type collyService[C entity.Crawlable[D], D model.Document] struct {
	collyCrawler  collector.CollyCrawler
	typedEsClient es.TypedEsClient[D]
	embedder      embedding.Embedder
	processSem    chan struct{}
	embedSem      chan struct{}
}

func InitCollyService[C entity.Crawlable[D], D model.Document](
	collyCrawler collector.CollyCrawler,
	typedEsClient es.TypedEsClient[D],
	embedder embedding.Embedder,
	processSemSize int,
	embedSemSize int,
) CollyService[C, D] {
	return &collyService[C, D]{
		collyCrawler:  collyCrawler,
		typedEsClient: typedEsClient,
		embedder:      embedder,
		processSem:    make(chan struct{}, processSemSize),
		embedSem:      make(chan struct{}, embedSemSize),
	}
}

func (cs *collyService[C, D]) CollyCrawler() collector.CollyCrawler {
	return cs.collyCrawler
}

func (cs *collyService[C, D]) TypedEsClient() es.TypedEsClient[D] {
	return cs.typedEsClient
}

func (cs *collyService[C, D]) Embedder() embedding.Embedder {
	return cs.embedder
}

func (cs *collyService[C, D]) Visit(url string) error {
	return cs.collyCrawler.Visit(url)
}

func (cs *collyService[C, D]) Wait() {
	cs.collyCrawler.Wait()
}

func (cs *collyService[C, D]) HandleResponse(ctx context.Context, toCrawlable func(body []byte) ([]C, error)) {
	//在colly的OnResponse回调中，只有在有响应时才会被调用。所以，信号量的获取尝试也只会在有响应时发生。
	cs.collyCrawler.OnResponse(func(r *colly.Response) {
		select {
		// 尝试向信号量通道发送一个空结构体
		case cs.processSem <- struct{}{}:
			// 如果发送成功，则注册一个函数在退出时从信号量通道接收（释放信号量）
			defer func() { <-cs.processSem }()
		default:
			// 如果上述发送操作无法立即完成（即通道已满）
			// 执行限流处理
			cs.handleRateLimit(r.Request)
			// 直接返回，不执行后续的数据处理
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
		cs.indexDocs(docs)
	})
}

func (cs *collyService[C, D]) HandleHTML(ctx context.Context, selector string, toCrawlable func(r *colly.HTMLElement) ([]C, error)) {
	//在colly的OnResponse回调中，只有在有响应时才会被调用。所以，信号量的获取尝试也只会在有响应时发生。
	cs.collyCrawler.OnHTML(selector, func(r *colly.HTMLElement) {
		// 尝试向信号量通道发送一个空结构体
		select {
		case cs.processSem <- struct{}{}:
			// 如果发送成功，则注册一个函数在退出时从信号量通道接收（释放信号量）
			defer func() { <-cs.processSem }()
		default:
			// 如果上述发送操作无法立即完成（即通道已满）
			// 执行限流处理
			cs.handleRateLimit(r.Request)
			// 直接返回，不执行后续的数据处理
			return
		}
		data, err := toCrawlable(r)
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
		cs.indexDocs(docs)
	})
}

func (cs *collyService[C, D]) indexDocs(docs []D) {

	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := cs.typedEsClient.BulkIndexDocsWithID(reqCtx, docs); err != nil {
		log.Printf("Bulk index error: %v", err)
		return
	}
}

func (cs *collyService[C, D]) handleRateLimit(r *colly.Request) {
	// 简单的丢弃策略，也可以实现排队或其他策略
	fmt.Printf("Rate limit hit, url: %s, discarding...\n", r.URL)
}
