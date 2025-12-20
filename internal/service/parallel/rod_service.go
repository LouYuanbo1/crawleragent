package service

import (
	"context"
	"log"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/domain/entity"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/crawler/parallel"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	"github.com/LouYuanbo1/crawleragent/param"
)

type rodParallelService[C entity.Crawlable[D], D model.Document] struct {
	parallelCrawler parallel.ParallelCrawler
	typedEsClient   es.TypedEsClient[D]
	embedder        embedding.Embedder
}

func InitRodParallelService[C entity.Crawlable[D], D model.Document](
	parallelCrawler parallel.ParallelCrawler,
	typedEsClient es.TypedEsClient[D],
	embedder embedding.Embedder,
) ParallelService[C, D] {
	return &rodParallelService[C, D]{
		parallelCrawler: parallelCrawler,
		typedEsClient:   typedEsClient,
		embedder:        embedder,
	}
}

func (rps *rodParallelService[C, D]) PerformAllUrlOperations(ctx context.Context, options []*param.UrlOperation) error {
	return rps.parallelCrawler.PerformAllUrlOperations(ctx, options)
}

func (rps *rodParallelService[C, D]) embeddingDocs(docs []D) {
	// 从配置中获取批量处理大小
	batchSizeEmbedding := rps.embedder.BatchSize()
	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	embeddingStrings := make([]string, 0, len(docs))
	for _, doc := range docs {
		embeddingStrings = append(embeddingStrings, doc.GetEmbeddingString())
	}
	var embeddingVectors [][]float32
	var err error
	if len(embeddingStrings) < batchSizeEmbedding {
		embeddingVectors, err = rps.embedder.Embed(reqCtx, embeddingStrings)
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
			embeddingVectors, err = rps.embedder.Embed(reqCtx, embeddingStrings[i:end])
			if err != nil {
				log.Printf("Embed error: %v", err)
			}
			for j := range embeddingVectors {
				docs[i+j].SetEmbedding(embeddingVectors[j])
				//log.Printf("Indexed doc %s with embedding len %d", docs[i+j].GetID(), len(docs[i+j].GetEmbedding()))
			}
		}
	}
}

func (rps *rodParallelService[C, D]) indexDocs(docs []D) {

	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := rps.typedEsClient.BulkIndexDocsWithID(reqCtx, docs); err != nil {
		log.Printf("Bulk index error: %v", err)
		return
	}
}

func (rps *rodParallelService[C, D]) ProcessRespChanWithIndexDocs(ctx context.Context, listener *param.ListenerConfig, toCrawlable func(body []byte) ([]C, error)) {
	go func() {
		for {
			select {
			case resps, ok := <-listener.RespChan:
				if !ok {
					log.Printf("响应通道已关闭,监听UrlPattern:%s\n", listener.UrlPattern)
					return
				}
				for _, resp := range resps {
					log.Printf("收到响应 (URL: %s,监听UrlPattern: %s,Length Body: %d)\n", resp.URL, listener.UrlPattern, len(resp.Body))
					crawlables, err := toCrawlable(resp.Body)
					if err != nil {
						log.Printf("处理响应体失败 (URL: %s,监听UrlPattern:%s): %v\n",
							resp.URL, listener.UrlPattern, err)
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
					rps.embeddingDocs(docs)
					rps.indexDocs(docs)
				}
			case <-ctx.Done():
				log.Printf("取消处理响应,监听UrlPattern:%s\n", listener.UrlPattern)
				return
			}
		}
	}()
}

func (rps *rodParallelService[C, D]) ProcessRespChan(ctx context.Context, listener *param.ListenerConfig) {
	go func() {
		for {
			select {
			case resps, ok := <-listener.RespChan:
				if !ok {
					log.Printf("响应通道已关闭,监听UrlPattern:%s\n", listener.UrlPattern)
					return
				}
				for _, resp := range resps {
					log.Printf("收到响应 (URL: %s,监听UrlPattern: %s,Length Body: %d)\n", resp.URL, listener.UrlPattern, len(resp.Body))
				}
			case <-ctx.Done():
				log.Printf("取消处理响应,监听UrlPattern:%s\n", listener.UrlPattern)
				return
			}
		}
	}()
}
