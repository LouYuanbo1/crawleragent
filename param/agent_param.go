package param

import "github.com/cloudwego/eino/components/prompt"

type Agent struct {
	IndexName string
	Prompt    map[string]*prompt.DefaultChatTemplate
}
