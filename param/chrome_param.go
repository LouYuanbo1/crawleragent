package param

// ChromeScroll 滚动爬取选项,用于配置滚动爬取的行为
type Scroll struct {
	Url                  string `json:"url"`
	ScrollTimes          int    `json:"scroll_times"`
	StandardSleepSeconds int    `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int    `json:"random_delay_seconds"`
}

type Click struct {
	Url                  string `json:"url"`
	Selector             string `json:"selector"`
	ClickTimes           int    `json:"click_times"`
	StandardSleepSeconds int    `json:"standard_sleep_seconds"`
	RandomDelaySeconds   int    `json:"random_delay_seconds"`
}
