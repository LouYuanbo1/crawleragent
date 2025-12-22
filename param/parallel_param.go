package param

import "github.com/LouYuanbo1/crawleragent/internal/infra/crawler/types"

type OperationType string

// 2. 定义常量，限制可能的值
const (
	OperationScroll OperationType = "scroll"
	OperationClick  OperationType = "click"
	OperationXClick OperationType = "xclick"
)

type ListenerConfig struct {
	UrlPattern string                       `json:"url_pattern"`
	RespChan   chan []types.NetworkResponse `json:"resp_chan"`
}

type UrlOperation struct {
	Url                  string              `json:"url"`
	OperationType        OperationType       `json:"operation_type"`
	NumActions           int                 `json:"num_actions"`
	StandardSleepSeconds int                 `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int                 `json:"random_delay_seconds"`
	ClickSelector        string              `json:"click_selector"`
	DataChan             chan types.DataChan `json:"data_chan"`
	Listener             *ListenerConfig     `json:"listener"`
}

func (uo *UrlOperation) IsValid() bool {
	switch uo.OperationType {
	case OperationScroll:
		return uo.Listener != nil
	case OperationClick, OperationXClick:
		return uo.Listener != nil && uo.ClickSelector != ""
	default:
		return false
	}
}
