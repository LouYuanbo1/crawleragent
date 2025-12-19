package embedding

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/cloudwego/eino-ext/components/embedding/ollama"
	"golang.org/x/sync/semaphore"
)

type embedder struct {
	model     *ollama.Embedder
	batchSize int
	embedSem  *semaphore.Weighted
}

// InitEmbedder 初始化嵌入器
func InitEmbedder(ctx context.Context, cfg *config.Config, embedSemSize int) (Embedder, error) {
	model, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
		Model:   cfg.Embedder.Model,
		BaseURL: cfg.Embedder.Host + ":" + strconv.Itoa(cfg.Embedder.Port),
	})
	if err != nil {
		return nil, err
	}
	embedSem := semaphore.NewWeighted(int64(embedSemSize))
	return &embedder{model: model, batchSize: cfg.Embedder.BatchSize, embedSem: embedSem}, nil
}

// BatchSize 返回批量处理大小
func (e *embedder) BatchSize() int {
	return e.batchSize
}

// Embed 将文本转换为向量表示
func (e *embedder) Embed(ctx context.Context, strings []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if len(strings) == 0 {
		return nil, nil
	}
	// 获取信号量（带超时）
	if err := e.embedSem.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("等待词嵌入信号量超时: %w", err)
	}
	defer e.embedSem.Release(1) // 保证释放

	embeddingVectors, err := e.model.EmbedStrings(ctx, strings)
	if err != nil {
		return nil, err
	}
	//EmbedStrings(ctx, strings)返回的是[][]float64类型的向量表示,需要转换为[][]float32类型(一般嵌入模型也是float32)
	allFloat32Vectors := make([][]float32, 0, len(embeddingVectors))
	for _, float64Vector := range embeddingVectors {

		float32Vectors := make([]float32, len(float64Vector)) // 每个 float32 占 4 字节
		for i, f := range float64Vector {
			float32Vectors[i] = float32(f)
		}
		allFloat32Vectors = append(allFloat32Vectors, float32Vectors)
	}
	return allFloat32Vectors, nil
}
