package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/chrome"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	"github.com/LouYuanbo1/crawleragent/param"
)

type chromedpService[C entity.Crawlable[D], D model.Document] struct {
	chromeCrawler chrome.ChromeCrawler
	typedEsClient es.TypedEsClient[D]
	embedder      embedding.Embedder
}

func InitChromedpService[C entity.Crawlable[D], D model.Document](
	chromeCrawler chrome.ChromeCrawler,
	typedEsClient es.TypedEsClient[D],
	embedder embedding.Embedder,
) ChromeService[C, D] {
	return &chromedpService[C, D]{
		chromeCrawler: chromeCrawler,
		typedEsClient: typedEsClient,
		embedder:      embedder,
	}
}

// 可能有大模型计算瓶颈或者内存瓶颈，可能要优化

func (cs *chromedpService[C, D]) ScrollStrategy(ctx context.Context, param *param.Scroll) error {
	log.Printf("开始滚动策略: %s", param.Url)

	// 初始化
	log.Printf("初始化浏览器并导航到: %s", param.Url)
	if err := cs.chromeCrawler.InitAndNavigate(param.Url); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	log.Printf("导航成功")

	// 执行多轮滚动

	if err := cs.chromeCrawler.PerformScrolling(param.ScrollTimes, param.StandardSleepSeconds, param.RandomDelaySeconds); err != nil {
		return fmt.Errorf("滚动失败: %w", err)
	}

	log.Printf("滚动爬取完成: %s", param.Url)
	return nil
}

func (cs *chromedpService[C, D]) ClickStrategy(ctx context.Context, param *param.Click) error {
	log.Printf("开始点击策略: %s", param.Url)

	// 初始化
	log.Printf("初始化浏览器并导航到: %s", param.Url)
	if err := cs.chromeCrawler.InitAndNavigate(param.Url); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	log.Printf("导航成功")

	if err := cs.chromeCrawler.PerformClick(param.Selector, param.ClickTimes, param.StandardSleepSeconds, param.RandomDelaySeconds); err != nil {
		return fmt.Errorf("点击失败: %w", err)
	}

	log.Printf("点击策略完成: %s", param.Url)
	return nil
}

func (cs *chromedpService[C, D]) SetNetworkListenerWithIndexDocs(ctx context.Context, urlPattern string, RespChanSize int, toCrawlable func(body []byte) ([]C, error)) {
	ctx, cancel := context.WithCancel(ctx)
	RespChan := make(chan []types.NetworkResponse, RespChanSize)
	cs.chromeCrawler.SetNetworkListener(urlPattern, RespChan)
	go func() {
		defer func() {
			close(RespChan)
			log.Printf("关闭监听: %s", urlPattern)
			cancel()
		}()
		for {
			select {
			case resps, ok := <-RespChan:
				if !ok {
					log.Printf("响应通道已关闭: %s", urlPattern)
					return
				}
				for _, resp := range resps {
					log.Printf("收到响应 (URL: %s)", resp.URL)
					crawlables, err := toCrawlable(resp.Body)
					if err != nil {
						log.Printf("处理响应体失败 (URL: %s): %v",
							resp.URL, err)
						continue
					}
					if len(crawlables) == 0 {
						continue
					}
					docs := make([]D, 0, len(crawlables))
					for _, crawlable := range crawlables {
						doc := crawlable.ToDocument()
						docs = append(docs, doc)
					}
					cs.embeddingDocs(docs)
					cs.indexDocs(docs)
				}
			case <-ctx.Done():
				log.Printf("取消监听: %s", urlPattern)
				return
			}
		}
	}()
}

func (cs *chromedpService[C, D]) SetNetworkListener(ctx context.Context, urlPattern string, RespChanSize int) {
	ctx, cancel := context.WithCancel(ctx)
	RespChan := make(chan []types.NetworkResponse, RespChanSize)
	cs.chromeCrawler.SetNetworkListener(urlPattern, RespChan)
	go func() {
		defer func() {
			close(RespChan)
			log.Printf("关闭监听: %s", urlPattern)
			cancel()
		}()
		for {
			select {
			case resps, ok := <-RespChan:
				if !ok {
					log.Printf("响应通道已关闭: %s", urlPattern)
					return
				}
				for _, resp := range resps {
					log.Printf("收到响应 (URL: %s)", resp.URL)
				}
			case <-ctx.Done():
				log.Printf("取消监听: %s", urlPattern)
				return
			}
		}
	}()
}

func (cs *chromedpService[C, D]) embeddingDocs(docs []D) {
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
