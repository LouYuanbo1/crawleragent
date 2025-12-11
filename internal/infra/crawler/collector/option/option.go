package option

type CollyRequest struct {
	UserAgent string            `json:"user_agent"`
	Headers   map[string]string `json:"headers"`
}
