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

type ListenerOperation struct {
	Url                  string          `json:"url"`
	OperationType        OperationType   `json:"operation_type"`
	Selector             string          `json:"selector"`
	NumActions           int             `json:"num_actions"`
	StandardSleepSeconds int             `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int             `json:"random_delay_seconds"`
	Listener             *ListenerConfig `json:"listener"`
}

func (lo *ListenerOperation) IsValid() bool {
	switch lo.OperationType {
	case OperationScroll:
		return lo.Listener != nil
	case OperationClick, OperationXClick:
		return lo.Listener != nil && lo.Selector != ""
	default:
		return false
	}
}
