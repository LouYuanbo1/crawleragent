package es

import (
	"context"

	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

/*
// 所有的文档结构体要实现这两个函数

	type Document interface {
		GetID() string
		GetIndex() string
	}
*/
type TypedEsClient[D model.Document] interface {
	GetClient() *elasticsearch.TypedClient
	CreateIndexWithMapping(ctx context.Context) error
	DeleteIndex(ctx context.Context) error
	IndexDocWithID(ctx context.Context, doc D) error
	BulkIndexDocsWithID(ctx context.Context, docs []D) error
	GetDoc(ctx context.Context, id string) (D, error)
	CountDocs(ctx context.Context) (int64, error)
	SearchDoc(ctx context.Context, query *types.Query, from, size int) ([]D, int64, error)
	UpdateDoc(ctx context.Context, doc D) error
	DeleteDoc(ctx context.Context, id string) error
	BulkDeleteDocs(ctx context.Context, ids []string) error
}
