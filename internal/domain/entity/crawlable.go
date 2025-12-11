package entity

import (
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
)

// 定义可爬取的实体接口
// 使用泛型定义可爬取的实体接口,增加扩展空间(如果可能的话)
// D是文档类型,必须实现model.Document接口
type Crawlable[D model.Document] interface {
	*RowBossJobData
	ToDocument() D
}
