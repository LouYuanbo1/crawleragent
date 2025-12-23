package types

type NetworkResponse struct {
	Url        string
	UrlPattern string
	Body       []byte
}

type HtmlContent struct {
	Url             string
	ContentSelector string
	Content         []string
}
