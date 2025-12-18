package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/LouYuanbo1/crawleragent/internal/domain/model"
	"github.com/LouYuanbo1/crawleragent/internal/infra/embedding"
	"github.com/LouYuanbo1/crawleragent/internal/infra/llm"
	"github.com/LouYuanbo1/crawleragent/internal/infra/persistence/es"
	"github.com/LouYuanbo1/crawleragent/param"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v9"
)

type State struct {
	IndexName     string
	TypedEsClient *elasticsearch.TypedClient
	Embedder      embedding.Embedder
}

type AgentService[D model.Document] interface {
	Stream(ctx context.Context, query string) error
	Invoke(ctx context.Context, query string) error
}

type agentService[D model.Document] struct {
	llm      llm.LLM
	es       es.TypedEsClient[D]
	embedder embedding.Embedder
	graph    compose.Runnable[map[string]any, map[string]any]
}

func InitAgentService[D model.Document](
	ctx context.Context,
	llm llm.LLM,
	es es.TypedEsClient[D],
	embedder embedding.Embedder,
	param *param.Agent,
) (AgentService[D], error) {
	graph, err := initAgentGraph(ctx, llm, es, embedder, param)
	if err != nil {
		return nil, fmt.Errorf("创建流程图失败: %w", err)
	}
	return &agentService[D]{llm: llm, es: es, embedder: embedder, graph: graph}, nil
}

// InitAgent 初始化AgentClient,根据options配置模型和节点
func initAgentGraph[D model.Document](
	ctx context.Context,
	llm llm.LLM,
	typedEsClient es.TypedEsClient[D],
	embedder embedding.Embedder,
	param *param.Agent,
) (compose.Runnable[map[string]any, map[string]any], error) {
	// 生成State,包含索引名称, TypedEsClient, Embedder 等状态信息
	genState := func(ctx context.Context) *State {
		return &State{
			IndexName:     param.IndexName,
			TypedEsClient: typedEsClient.GetClient(),
			Embedder:      embedder,
		}
	}

	fmt.Printf("genState: %+v\n", genState(ctx))
	// 初始化Compose图,设置全局状态生成函数
	graph := compose.NewGraph[map[string]any, map[string]any](compose.WithGenLocalState(genState))
	// 添加意图检测节点,用于识别用户查询的意图,当用户输入以查询模式或搜索模式开头时,将意图设置为"retriever",
	// 使用爬取的信息做RAG增强
	err := graph.AddLambdaNode("intentDetection", IntentDetection())
	if err != nil {
		log.Printf("Error adding lambda node: %v", err)
		return nil, err
	}
	// 添加检索节点,用于根据用户查询意图,从索引中检索相关文档
	err = graph.AddLambdaNode("retriever", Retriever())
	if err != nil {
		log.Printf("Error adding lambda node: %v", err)
		return nil, err
	}
	// 添加搜索模式提示节点,用于根据用户查询意图,生成搜索模式的提示
	err = graph.AddChatTemplateNode("searchModePrompt", param.Prompt["searchMode"])
	if err != nil {
		log.Printf("Error adding prompt template node: %v", err)
		return nil, err
	}
	// 添加聊天模式提示节点,用于根据用户查询意图,生成聊天模式的提示
	err = graph.AddChatTemplateNode("chatModePrompt", param.Prompt["chatMode"])
	if err != nil {
		log.Printf("Error adding prompt template node: %v", err)
		return nil, err
	}

	err = graph.AddChatModelNode("llm", llm.Model(), compose.WithOutputKey("finalResponse"))
	if err != nil {
		log.Printf("Error adding LLM node: %v", err)
		return nil, err
	}

	err = graph.AddEdge(compose.START, "intentDetection")
	if err != nil {
		log.Printf("Error adding edge: %v", err)
		return nil, err
	}

	err = graph.AddBranch("intentDetection", compose.NewGraphBranch(BranchCondition, map[string]bool{
		"retriever":      true,
		"chatModePrompt": true,
	}))
	if err != nil {
		log.Printf("Error adding branch: %v", err)
		return nil, err
	}

	err = graph.AddEdge("retriever", "searchModePrompt")
	if err != nil {
		log.Printf("Error adding edge: %v", err)
		return nil, err
	}

	err = graph.AddEdge("searchModePrompt", "llm")
	if err != nil {
		log.Printf("Error adding edge: %v", err)
		return nil, err
	}

	err = graph.AddEdge("chatModePrompt", "llm")
	if err != nil {
		log.Printf("Error adding edge: %v", err)
		return nil, err
	}

	err = graph.AddEdge("llm", compose.END)
	if err != nil {
		log.Printf("Error adding edge: %v", err)
		return nil, err
	}

	compiledGraph, _ := graph.Compile(ctx)
	return compiledGraph, nil

}

func (as *agentService[D]) Invoke(ctx context.Context, query string) error {
	result, err := as.graph.Invoke(ctx, map[string]any{
		"query": query,
	})
	if err != nil {
		log.Printf("Failed to invoke graph: %v", err)
		return err
	}

	// 从结果中提取最终回复
	if finalResponse, ok := result["finalResponse"].(*schema.Message); ok {
		fmt.Println(finalResponse.Content)
		return nil
	}

	fmt.Println("抱歉，我无法理解您的请求。")
	return nil
}

func (as *agentService[D]) Stream(ctx context.Context, query string) error {
	result, err := as.graph.Stream(ctx, map[string]any{
		"query": query,
	})
	if err != nil {
		log.Printf("Failed to invoke graph: %v", err)
		return err
	}

	for {
		chunk, err := result.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Printf("\n\n")
			break
		}
		if err != nil {
			log.Printf("Error receiving chunk: %v", err)
			return err
		}
		if msg, ok := chunk["finalResponse"].(*schema.Message); ok {
			fmt.Print(msg.Content)
		}
	}
	return nil
}
