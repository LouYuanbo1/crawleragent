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
	UrlPatterns []string                    `json:"url_patterns" jsonschema:"description=url patterns to match"`
	ListenerCh  chan *types.NetworkResponse `json:"listener" jsonschema:"description=listener config"`
}

type HtmlContentConfig struct {
	ContentSelectors []string                `json:"content_selector" jsonschema:"description=css selector to operate"`
	HtmlContentsCh   chan *types.HtmlContent `json:"html_contents" jsonschema:"description=html content config"`
}

type UrlOperation struct {
	Url                  string             `json:"url"`
	OperationType        OperationType      `json:"operation_type"`
	NumActions           int                `json:"num_actions"`
	StandardSleepSeconds int                `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int                `json:"random_delay_seconds"`
	ClickSelector        string             `json:"click_selector"`
	ListenerConfig       *ListenerConfig    `json:"listener_config"`
	HtmlContentConfig    *HtmlContentConfig `json:"html_content_config"`
}

func (uo *UrlOperation) IsValid() bool {
	if uo.Url == "" ||
		uo.OperationType == "" ||
		uo.NumActions <= 0 ||
		uo.StandardSleepSeconds <= 0 ||
		uo.RandomDelaySeconds <= 0 ||
		(uo.ListenerConfig == nil &&
			uo.HtmlContentConfig == nil) {
		return false
	}
	switch uo.OperationType {
	case OperationScroll:
		return true
	case OperationClick, OperationXClick:
		return uo.ClickSelector != ""
	default:
		return false
	}
}
