package es

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/LouYuanbo1/crawleragent/internal/config"
	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esutil"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/xuri/excelize/v2"
	"golang.org/x/sync/semaphore"
)

type typedEsClient[D model.Document] struct {
	client *elasticsearch.TypedClient
	// 特别说明：这个实例仅用于获取配置信息，不用于存储数据
	// Instance used for getting schema/configuration, not for data storage
	schemaDoc D
	esSem     *semaphore.Weighted
}

func InitTypedEsClient[D model.Document](cfg *config.Config, esSemSize int) (TypedEsClient[D], error) {
	typedClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Username: cfg.Elasticsearch.Username,
		Password: cfg.Elasticsearch.Password,
		Addresses: []string{
			cfg.Elasticsearch.Address,
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			// 跳过TLS验证（仅在开发环境中使用）
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Elasticsearch client: %s", err)
	}
	// 初始化信号量
	esSem := semaphore.NewWeighted(int64(esSemSize))

	return &typedEsClient[D]{client: typedClient, esSem: esSem}, nil
}

func (tec *typedEsClient[D]) GetClient() *elasticsearch.TypedClient {
	return tec.client
}

func (tec *typedEsClient[D]) CreateIndexWithMapping(ctx context.Context) error {
	// 检查索引是否已存在
	index := tec.schemaDoc.GetIndex()
	mapping := tec.schemaDoc.GetTypeMapping()
	exists, err := tec.client.Indices.Exists(index).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check index existence in es: %s", err)
	}
	if exists {
		log.Printf("Index %s already exists, skip create", index)
		getMappingResponse, err := tec.client.Indices.GetMapping().Index(index).Do(ctx)
		if err != nil {
			log.Printf("Failed to get index mapping: %s", err)
		} else {
			// 将mapping转换为JSON格式打印
			//json.MarshalIndent
			// 格式化格式：生成人类可读的、带缩进和换行的 JSON
			// 适合场景：日志记录、调试、配置文件、人类阅读等
			// 第一个参数 "" (prefix) - 行前缀
			// 作用：指定每一行 JSON 数据开头的前缀字符串
			// 第二个参数 " " (indent) - 缩进字符
			// 作用：指定每一级嵌套使用的缩进字符串
			jsonData, err := json.MarshalIndent(getMappingResponse, "", "  ")
			if err != nil {
				log.Printf("Failed to marshal mapping to JSON: %s", err)
			} else {
				log.Printf("Index mapping for %s:\n%s", index, string(jsonData))
			}
		}
		return nil
	}

	if mapping == nil {
		_, err = tec.client.Indices.Create(index).Do(ctx)
	} else {
		_, err = tec.client.Indices.Create(index).Mappings(mapping).Do(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to create index in es: %s", err)
	}
	return nil
}

func (tec *typedEsClient[D]) DeleteIndex(ctx context.Context) error {
	index := tec.schemaDoc.GetIndex()
	_, err := tec.client.Indices.Delete(index).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete index in es: %s", err)
	}
	return nil
}

func (tec *typedEsClient[D]) IndexDocWithID(ctx context.Context, doc D) error {
	_, err := tec.client.Index(tec.schemaDoc.GetIndex()).
		Id(doc.GetID()).
		Document(doc).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to index doc to es: %s", err)
	}
	return nil
}

func (tec *typedEsClient[D]) BulkIndexDocsWithID(ctx context.Context, docs []D) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// 获取信号量（带超时）
	if err := tec.esSem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("等待ES索引信号量超时: %w", err)
	}
	defer tec.esSem.Release(1) // 保证释放

	if len(docs) == 0 {
		return nil
	}
	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:         tec.schemaDoc.GetIndex(), // 目标索引名称
		Client:        tec.client,               // Elasticsearch 客户端
		NumWorkers:    2,                        // 并发工作协程数
		FlushBytes:    5 * 1024 * 1024,          // 5MB 时自动刷新
		FlushInterval: 30 * time.Second,         // 30秒自动刷新
		// 可选：错误处理回调
		OnError: func(ctx context.Context, err error) {
			log.Printf("Bulk indexer error: %s", err)
		},
	})
	if err != nil {
		log.Printf("Error creating bulk indexer: %s", err)
	}

	// 4. 添加文档到批量索引器
	for _, doc := range docs {

		data, err := json.Marshal(doc)
		if err != nil {
			log.Printf("Error marshaling document: %s", err)
		}

		err = bi.Add(ctx, esutil.BulkIndexerItem{
			Action:     "index",                         // 操作类型：index, create, update, delete
			DocumentID: doc.GetID(),                     // 文档ID
			Body:       strings.NewReader(string(data)), // 文档内容
			OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
				//fmt.Printf("Successfully indexed document %s\n", item.DocumentID)
			},
			OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
				if err != nil {
					log.Printf("Error indexing document %s: %s", item.DocumentID, err)
				} else {
					log.Printf("Failed to index document %s: %s", item.DocumentID, res.Error.Reason)
				}
			},
		})
		if err != nil {
			log.Printf("Unexpected error: %s", err)
		}
	}

	// 5. 刷新并关闭批量索引器（确保所有文档都被处理）
	if err := bi.Close(ctx); err != nil {
		log.Printf("Error closing bulk indexer: %s", err)
	}

	// 6. 获取统计信息
	stats := bi.Stats()
	fmt.Printf("Bulk indexing completed:\n")
	fmt.Printf("  Indexed: %d documents\n", stats.NumIndexed)
	return nil
}

func (tec *typedEsClient[D]) GetDoc(ctx context.Context, id string) (D, error) {
	index := tec.schemaDoc.GetIndex()
	resp, err := tec.client.Get(index, id).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get doc from es")
	}
	if !resp.Found {
		log.Println("未找到id对应doc结果.id: ", id)
		return nil, nil
	}
	var doc D
	if err := json.Unmarshal(resp.Source_, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal source: %s", err)
	}
	fmt.Printf("Parsed Document - ID: %s, Index: %s\n", doc.GetID(), doc.GetIndex())
	return doc, nil
}

// 使用 []D 作为返回类型
func (tec *typedEsClient[D]) SearchDoc(ctx context.Context, query *types.Query, from, size int) ([]D, int64, error) {
	resp, err := tec.client.Search().
		Index(tec.schemaDoc.GetIndex()).
		Query(query).
		From(from).
		Size(size).
		Do(ctx)

	if err != nil {
		return nil, 0, fmt.Errorf("搜索失败: %w", err)
	}

	// 预分配切片容量，避免多次扩容
	results := make([]D, 0, len(resp.Hits.Hits))

	for _, hit := range resp.Hits.Hits {
		// 为每个文档分配新的 D 实例,使用泛型确定绑定结构体
		var doc D
		if err := json.Unmarshal(hit.Source_, &doc); err != nil {
			continue
		}
		// 将 doc 的地址存入切片
		results = append(results, doc)
	}

	return results, resp.Hits.Total.Value, nil
}

func (tec *typedEsClient[D]) CountDocs(ctx context.Context) (int64, error) {
	resp, err := tec.client.Count().Index(tec.schemaDoc.GetIndex()).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count docs in es: %s", err)
	}
	return resp.Count, nil
}

// 支持部分更新
func (tec *typedEsClient[D]) UpdateDoc(ctx context.Context, doc D) error {
	_, err := tec.client.Update(tec.schemaDoc.GetIndex(), doc.GetID()).
		Doc(doc).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to update doc in es: %s", err)
	}
	return nil
}

func (tec *typedEsClient[D]) DeleteDoc(ctx context.Context, id string) error {
	_, err := tec.client.Delete(tec.schemaDoc.GetIndex(), id).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete doc from es: %s", err)
	}
	return nil
}

func (tec *typedEsClient[D]) BulkDeleteDocs(ctx context.Context, ids []string) error {
	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:         tec.schemaDoc.GetIndex(), // 目标索引名称
		Client:        tec.client,               // Elasticsearch 客户端
		NumWorkers:    2,                        // 并发工作协程数
		FlushBytes:    5 * 1024 * 1024,          // 5MB 时自动刷新
		FlushInterval: 30 * time.Second,         // 30秒自动刷新
		// 可选：错误处理回调
		OnError: func(ctx context.Context, err error) {
			log.Printf("Bulk indexer error: %s", err)
		},
	})
	if err != nil {
		log.Fatalf("Error creating bulk indexer: %s", err)
	}

	// 4. 添加文档到批量索引器
	for _, id := range ids {

		err = bi.Add(ctx, esutil.BulkIndexerItem{
			Action:     "delete", // 操作类型：index, create, update, delete
			DocumentID: id,       // 文档ID
			OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
				fmt.Printf("Successfully deleted document %s\n", item.DocumentID)
			},
			OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
				if err != nil {
					log.Printf("Error deleting document %s: %s", item.DocumentID, err)
				} else {
					log.Printf("Failed to delete document %s: %s", item.DocumentID, res.Error.Reason)
				}
			},
		})
		if err != nil {
			log.Fatalf("Unexpected error: %s", err)
		}
	}

	// 5. 刷新并关闭批量索引器（确保所有文档都被处理）
	if err := bi.Close(ctx); err != nil {
		log.Fatalf("Error closing bulk indexer: %s", err)
	}

	// 6. 获取统计信息
	stats := bi.Stats()
	fmt.Printf("Bulk indexing completed:\n")
	fmt.Printf("  Deleted: %d documents\n", stats.NumDeleted)
	return nil
}

func (tec *typedEsClient[D]) ToExcel(ctx context.Context, filename string, sortFields []string, size int) error {
	var f *excelize.File
	var err error
	f, err = excelize.OpenFile(filename)
	if err != nil {
		log.Printf("打开excel文件失败,创建新文件: %s", filename)
		f = excelize.NewFile()
		f.SetSheetName("Sheet1", "Data")
	}

	defer func() {
		if err := f.SaveAs(filename); err != nil {
			log.Printf("保存Excel文件失败: %v", err)
		}
		if err := f.Close(); err != nil {
			log.Printf("关闭Excel文件失败: %v", err)
		}
	}()

	/*

		mapSortFields := make(map[string]types.FieldSort, len(sortFields))

		for _, field := range sortFields {
			mapSortFields[field] = types.FieldSort{
				Order: &sortorder.Desc,
			}
		}
		sortOptions := types.SortOptions{
			SortOptions: mapSortFields,
		}

	*/
	resp, err := tec.client.Search().
		Index(tec.schemaDoc.GetIndex()).
		Query(&types.Query{
			MatchAll: &types.MatchAllQuery{},
		}).
		//Sort(&sortOptions).
		Scroll("1m").
		Size(size).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to scroll docs in es: %s", err)
	}

	// 获取工作表名
	sheetName := "Data"
	// 获取当前最大行数（如果是新文件，从第1行开始）
	rowNum := 1
	if rows, err := f.GetRows(sheetName); err == nil && len(rows) > 0 {
		rowNum = len(rows) + 1
	}

	// 记录处理的总文档数
	totalProcessed := 0
	scrollID := resp.ScrollId_

	// 处理第一批数据
	for {
		if len(resp.Hits.Hits) == 0 {
			break
		}

		// 处理当前批次的文档
		for _, hit := range resp.Hits.Hits {
			// 解析文档数据
			var doc D
			if err := json.Unmarshal(hit.Source_, &doc); err != nil {
				log.Printf("解析文档失败: %v", err)
				continue
			}

			// 将文档写入Excel
			// 这里需要根据D的类型来设置具体的列
			// 假设D有ToMap()方法或者使用反射
			if err := tec.writeDocToExcel(f, sheetName, rowNum, doc); err != nil {
				log.Printf("写入Excel失败: %v", err)
			}

			rowNum++
			totalProcessed++

			// 如果指定了最大数量且已达到，则停止
			if size > 0 && totalProcessed >= size {
				break
			}
		}

		// 检查是否已达到指定数量
		if size > 0 && totalProcessed >= size {
			break
		}

		// 获取下一批数据
		resp, err := tec.client.Scroll().
			ScrollId(*scrollID).
			Do(ctx)

		if err != nil {
			// 尝试清除scroll
			tec.client.ClearScroll().ScrollId(*scrollID).Do(ctx)
			if err.Error() == "EOF" {
				// 所有数据已读取完毕
				break
			}
			return fmt.Errorf("failed to scroll: %w", err)
		}

		// 检查是否还有数据
		if len(resp.Hits.Hits) == 0 {
			break
		}

		scrollID = resp.ScrollId_
	}

	// 清理scroll
	if scrollID != nil {
		tec.client.ClearScroll().ScrollId(*scrollID).Do(ctx)
	}

	log.Printf("成功处理 %d 个文档到Excel文件: %s", totalProcessed, filename)
	return nil
}

// 辅助函数：将文档写入Excel
func (tec *typedEsClient[D]) writeDocToExcel(f *excelize.File, sheetName string, row int, doc D) error {
	// 这里需要根据D的具体类型来写入Excel
	// 你可以使用反射或者为D类型实现一个ToRow()方法

	// 示例：使用反射获取字段值
	val := reflect.ValueOf(doc)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 假设是结构体
	if val.Kind() == reflect.Struct {
		//typ := val.Type()
		col := 1

		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			//fieldName := typ.Field(i).Name

			// 获取字段值
			var cellValue string
			switch field.Kind() {
			case reflect.String:
				cellValue = field.String()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				cellValue = strconv.FormatInt(field.Int(), 10)
			case reflect.Float32, reflect.Float64:
				cellValue = strconv.FormatFloat(field.Float(), 'f', -1, 64)
			case reflect.Bool:
				cellValue = strconv.FormatBool(field.Bool())
			default:
				// 尝试转换为字符串
				if field.CanInterface() {
					cellValue = fmt.Sprintf("%v", field.Interface())
				}
			}

			// 写入Excel
			cell, _ := excelize.CoordinatesToCellName(col, row)
			f.SetCellValue(sheetName, cell, cellValue)
			col++
		}
	}

	return nil
}
