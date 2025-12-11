package crawler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/chrome"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	"github.com/LouYuanbo1/crawleragent/internal/service/crawler/param"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type ChromedpService[C entity.Crawlable[D], D model.Document] interface {
	ChromedpCrawler() chrome.ChromedpCrawler
	TypedEsClient() es.TypedEsClient[D]
	Embedder() embedding.Embedder
	ScrollCrawl(ctx context.Context, params *param.ChromeScroll, batchSizeEmbedding int, toCrawlable func(body []byte) ([]C, error)) error
}

type chromedpService[C entity.Crawlable[D], D model.Document] struct {
	chromedpCrawler chrome.ChromedpCrawler
	typedEsClient   es.TypedEsClient[D]
	embedder        embedding.Embedder
	processSem      chan struct{}
	embedSem        chan struct{}
}

func InitChromedpService[C entity.Crawlable[D], D model.Document](
	chromedpCrawler chrome.ChromedpCrawler,
	typedEsClient es.TypedEsClient[D],
	embedder embedding.Embedder,
	processSemSize int,
	embedSemSize int,
) ChromedpService[C, D] {
	return &chromedpService[C, D]{
		chromedpCrawler: chromedpCrawler,
		typedEsClient:   typedEsClient,
		embedder:        embedder,
		processSem:      make(chan struct{}, processSemSize),
		embedSem:        make(chan struct{}, embedSemSize),
	}
}

func (cs *chromedpService[C, D]) ChromedpCrawler() chrome.ChromedpCrawler {
	return cs.chromedpCrawler
}

func (cs *chromedpService[C, D]) TypedEsClient() es.TypedEsClient[D] {
	return cs.typedEsClient
}

func (cs *chromedpService[C, D]) Embedder() embedding.Embedder {
	return cs.embedder
}

// 可能有大模型计算瓶颈或者内存瓶颈，可能要优化

func (cs *chromedpService[C, D]) ScrollCrawl(ctx context.Context, params *param.ChromeScroll, batchSizeEmbedding int, toCrawlable func(body []byte) ([]C, error)) error {
	log.Printf("开始滚动爬取: %s", params.Url)

	// 设置监听器
	cs.SetupNetworkListener(params.UrlPattern, toCrawlable)

	// 初始化
	log.Printf("初始化浏览器并导航到: %s", params.Url)
	if err := cs.chromedpCrawler.InitAndNavigate(params.Url); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	log.Printf("导航成功")

	// 执行多轮滚动
	for i := range params.Rounds {
		log.Printf("执行第 %d/%d 轮滚动", i+1, params.Rounds)

		if err := cs.chromedpCrawler.ResetAndScroll(params.ScrollTimes, params.StandardSleepSeconds, params.RandomDelaySeconds); err != nil {
			return fmt.Errorf("第 %d 轮滚动失败: %w", i+1, err)
		}

		log.Printf("第 %d 轮滚动完成", i+1)
	}

	log.Printf("滚动爬取完成: %s", params.Url)
	return nil
}

func (cs *chromedpService[C, D]) SetupNetworkListener(urlPattern string, toCrawlable func(body []byte) ([]C, error)) {
	pageCtx := cs.chromedpCrawler.PageContext()
	reqCache := cs.chromedpCrawler.RequestCache()
	chromedp.ListenTarget(pageCtx, func(ev any) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			resp := ev.Response
			if strings.Contains(resp.URL, urlPattern) {
				fmt.Printf("请求ID: %s\n", ev.RequestID)
				fmt.Printf("检测到目标API响应: %s\n", resp.URL)
				fmt.Printf("响应状态码: %d\n", resp.Status)
				reqCache.Store(ev.RequestID, resp.URL)
			}

		case *network.EventLoadingFinished:
			// 当请求加载完成时获取响应体
			if cachedURL, ok := reqCache.Load(ev.RequestID); ok {
				// 类型断言，因为Load返回any类型
				if urlStr, ok := cachedURL.(string); ok {
					if strings.Contains(urlStr, urlPattern) {
						// 处理完成后删除
						reqCache.Delete(ev.RequestID)
						go cs.fetchAndProcessResponse(ev.RequestID, urlStr, toCrawlable)
					}
				}
			}
		}
	})
}

func (cs *chromedpService[C, D]) fetchAndProcessResponse(requestID network.RequestID, cachedURL string, toCrawlable func(body []byte) ([]C, error)) {
	// 1. 获取信号量（控制并发）
	select {
	case cs.processSem <- struct{}{}:
		// 成功获取信号量
	default:
		// 并发数已达上限，丢弃此请求
		log.Printf("并发数已达上限(%d)，丢弃请求: %s", cap(cs.processSem), cachedURL)
		return
	}
	defer func() { <-cs.processSem }() // 释放信号量

	pageCtx := cs.chromedpCrawler.PageContext()
	c := chromedp.FromContext(pageCtx)
	responseBodyParams := network.GetResponseBody(requestID)
	ctx := cdp.WithExecutor(pageCtx, c.Target)
	body, err := responseBodyParams.Do(ctx)
	if err != nil {
		log.Printf("获取响应体失败 (RequestID: %s): %v",
			requestID, err)
		return
	}

	fmt.Printf("成功获取响应体 (URL: %s, RequestID: %s, 大小: %d bytes)\n", cachedURL, requestID, len(body))
	// 调用处理函数处理响应体
	crawlables, err := toCrawlable(body)
	if err != nil {
		log.Printf("处理响应体失败 (RequestID: %s): %v",
			requestID, err)
		return
	}
	if len(crawlables) == 0 {
		return
	}
	docs := make([]D, 0, len(crawlables))
	for _, crawlable := range crawlables {
		doc := crawlable.ToDocument()
		docs = append(docs, doc)
	}
	cs.embeddingDocs(docs)
	cs.indexDocs(docs)
}

func (cs *chromedpService[C, D]) embeddingDocs(docs []D) {
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

func (cs *chromedpService[C, D]) indexDocs(docs []D) {

	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := cs.typedEsClient.BulkIndexDocsWithID(reqCtx, docs); err != nil {
		log.Printf("Bulk index error: %v", err)
		return
	}
}
