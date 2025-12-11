package option

type CollyCrawler struct {
	UserAgent string            `json:"user_agent"`
	Headers   map[string]string `json:"headers"`
}
