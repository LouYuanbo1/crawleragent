package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/cloudwego/eino/compose"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

// IntentDetection 意图检测节点,用于识别用户查询的意图,当用户输入以查询模式或搜索模式开头时,将意图设置为"retriever",
// 否则将意图设置为"chatModePrompt"
func IntentDetection() *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, state map[string]any) (map[string]any, error) {
		query, ok := state["query"].(string)
		if !ok {
			return nil, errors.New("query not found in state")
		}
		isSearchMode := strings.HasPrefix(query, "查询模式") || strings.HasPrefix(query, "搜索模式")
		if isSearchMode {
			state["isSearchMode"] = true
		} else {
			state["isSearchMode"] = false
		}
		println("isSearchMode: ", state["isSearchMode"].(bool))
		return state, nil
	})
}

// BranchCondition 分支条件节点,根据用户查询意图,选择下一个节点,当用户输入以查询模式或搜索模式开头时,将选择"retriever"节点,
// 否则将选择"chatModePrompt"节点
func BranchCondition(ctx context.Context, state map[string]any) (string, error) {
	isSearchMode, ok := state["isSearchMode"].(bool)
	if !ok {
		return "", errors.New("isSearchMode not found in state")
	}
	if isSearchMode {
		return "retriever", nil
	}
	return "chatModePrompt", nil
}

// Retriever 检索节点,用于根据用户查询意图,从索引中检索相关文档
func Retriever[D model.Document]() *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, state map[string]any) (map[string]any, error) {
		query, ok := state["query"].(string)
		if !ok {
			return nil, errors.New("query not found in state")
		}
		fmt.Printf("query: %s", query)
		var embeddings [][]float32
		var err error
		err = compose.ProcessState(ctx, func(ctx context.Context, s *State) error {
			embeddings, err = s.Embedder.Embed(ctx, []string{query})
			if err != nil {
				return err
			}
			embedding := embeddings[0]
			K := 5
			numCandidates := 100
			searchResp, err := s.TypedEsClient.Search().Index(s.IndexName).
				Request(&search.Request{
					Knn: []types.KnnSearch{
						{
							Field:         "embedding",
							QueryVector:   embedding,
							K:             &K,
							NumCandidates: &numCandidates,
						},
					},
				}).Do(ctx)
			if err != nil {
				return err
			}
			var Builder strings.Builder
			Builder.WriteString("参考文档(JSON格式):\n\n")
			for i, hit := range searchResp.Hits.Hits {
				Builder.WriteString(fmt.Sprintf("文档%d:\n", i+1))
				Builder.WriteString(string(hit.Source_))
				Builder.WriteString("\n\n")
			}
			state["referenceDocs"] = Builder.String()
			return nil
		})
		if err != nil {
			return nil, err
		}

		return state, nil
	})
}
