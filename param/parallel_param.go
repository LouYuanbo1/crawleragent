package param

import "github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"

type OperationType string

// 2. 定义常量，限制可能的值
const (
	OperationScroll OperationType = "scroll"
	OperationClick  OperationType = "click"
	OperationXClick OperationType = "xclick"
)

type Listener struct {
	UrlPattern string                       `json:"url_pattern"`
	RespChan   chan []types.NetworkResponse `json:"resp_chan"`
}

type URLOperation struct {
	Url                  string                       `json:"url"`
	OperationType        OperationType                `json:"operation_type"`
	Selector             string                       `json:"selector"`
	DataChan             chan []types.NetworkResponse `json:"data_chan"`
	Times                int                          `json:"times"`
	StandardSleepSeconds int                          `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int                          `json:"random_delay_seconds"`
	Listener             *Listener                    `json:"listener"`
}
