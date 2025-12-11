package model

import (
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

type Document interface {
	*BossJobDoc
	GetID() string
	GetIndex() string
	GetTypeMapping() *types.TypeMapping
	GetEmbeddingString() string
	SetEmbedding(embedding []float32)
	GetEmbedding() []float32
}
