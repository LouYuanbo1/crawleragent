package embedding

import (
	"context"
	"strconv"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/cloudwego/eino-ext/components/embedding/ollama"
)

type embedder struct {
	model     *ollama.Embedder
	batchSize int
}

// InitEmbedder 初始化嵌入器
func InitEmbedder(ctx context.Context, cfg *config.Config) (Embedder, error) {
	model, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
		Model:   cfg.Embedder.Model,
		BaseURL: cfg.Embedder.Host + ":" + strconv.Itoa(cfg.Embedder.Port),
	})
	if err != nil {
		return nil, err
	}
	return &embedder{model: model, batchSize: cfg.Embedder.BatchSize}, nil
}

// BatchSize 返回批量处理大小
func (e *embedder) BatchSize() int {
	return e.batchSize
}

// Embed 将文本转换为向量表示
func (e *embedder) Embed(ctx context.Context, strings []string) ([][]float32, error) {
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
