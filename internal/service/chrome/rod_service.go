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
	"github.com/LouYuanbo1/crawleragent/internal/service/chrome/param"
)

type rodService[C entity.Crawlable[D], D model.Document] struct {
	chromeCrawler chrome.ChromeCrawler
	typedEsClient es.TypedEsClient[D]
	embedder      embedding.Embedder
}

func InitRodService[C entity.Crawlable[D], D model.Document](
	chromeCrawler chrome.ChromeCrawler,
	typedEsClient es.TypedEsClient[D],
	embedder embedding.Embedder,
) ChromeService[C, D] {
	return &rodService[C, D]{
		chromeCrawler: chromeCrawler,
		typedEsClient: typedEsClient,
		embedder:      embedder,
	}
}

func (cs *rodService[C, D]) ScrollStrategy(ctx context.Context, params *param.Scroll) error {
	log.Printf("开始滚动策略: %s", params.Url)

	// 初始化
	log.Printf("初始化浏览器并导航到: %s", params.Url)
	if err := cs.chromeCrawler.InitAndNavigate(params.Url); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	log.Printf("导航成功")

	// 执行多轮滚动
	for i := range params.Rounds {
		log.Printf("执行第 %d/%d 轮滚动", i+1, params.Rounds)

		if err := cs.chromeCrawler.PerformScrolling(params.ScrollTimes, params.StandardSleepSeconds, params.RandomDelaySeconds); err != nil {
			return fmt.Errorf("第 %d 轮滚动失败: %w", i+1, err)
		}

		log.Printf("第 %d 轮滚动完成", i+1)
	}

	log.Printf("滚动爬取完成: %s", params.Url)
	return nil
}

func (cs *chromedpService[C, D]) ClickStrategy(ctx context.Context, params *param.Click) error {
	log.Printf("开始点击策略: %s", params.Url)

	// 初始化
	log.Printf("初始化浏览器并导航到: %s", params.Url)
	if err := cs.chromeCrawler.InitAndNavigate(params.Url); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	log.Printf("导航成功")

	// 执行多轮滚动
	for i := range params.Rounds {
		log.Printf("执行第 %d/%d 轮点击", i+1, params.Rounds)

		if err := cs.chromeCrawler.PerformClick(params.Selector, params.ClickTimes, params.StandardSleepSeconds, params.RandomDelaySeconds); err != nil {
			return fmt.Errorf("第 %d 轮点击失败: %w", i+1, err)
		}

		log.Printf("第 %d 轮点击完成", i+1)
	}

	log.Printf("点击策略完成: %s", params.Url)
	return nil
}

func (cs *rodService[C, D]) SetNetworkListenerWithIndexDocs(ctx context.Context, urlPattern string, RespChanSize int, toCrawlable func(body []byte) ([]C, error)) {
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

func (cs *rodService[C, D]) SetNetworkListener(ctx context.Context, urlPattern string, RespChanSize int) {
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

func (rs *rodService[C, D]) embeddingDocs(docs []D) {
	// 从配置中获取批量处理大小
	batchSizeEmbedding := rs.embedder.BatchSize()
	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	embeddingStrings := make([]string, 0, len(docs))
	for _, doc := range docs {
		embeddingStrings = append(embeddingStrings, doc.GetEmbeddingString())
	}
	var embeddingVectors [][]float32
	var err error
	if len(embeddingStrings) < batchSizeEmbedding {
		embeddingVectors, err = rs.embedder.Embed(reqCtx, embeddingStrings)
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
			embeddingVectors, err = rs.embedder.Embed(reqCtx, embeddingStrings[i:end])
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

func (rs *rodService[C, D]) indexDocs(docs []D) {

	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := rs.typedEsClient.BulkIndexDocsWithID(reqCtx, docs); err != nil {
		log.Printf("Bulk index error: %v", err)
		return
	}
}
