package param

type OperationType string

// 2. 定义常量，限制可能的值
const (
	OperationScroll OperationType = "scroll"
	OperationClick  OperationType = "click"
)

type URLOperation struct {
	Url                  string        `json:"url"`
	OperationType        OperationType `json:"operation_type"`
	Selector             string        `json:"selector"`
	Times                int           `json:"times"`
	StandardSleepSeconds int           `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int           `json:"random_delay_seconds"`
}
