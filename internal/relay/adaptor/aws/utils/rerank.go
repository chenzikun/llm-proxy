package utils

import (
	"context"
	"fmt"
	"github.com/zicorn/llm-proxy/pkg/common/logger"

	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
)

type RerankResultItem struct {
	Index          int32   `json:"index"`
	RelevanceScore float32 `json:"relevance_score"`
}

type RerankResult struct {
	Results []RerankResultItem `json:"results"`
	Id      string             `json:"id"`
	Meta    struct {
		ApiVersion struct {
			Version        string `json:"version"`
			IsExperimental bool   `json:"is_experimental"`
		} `json:"api_version"`
		BilledUnits struct {
			SearchUnits int `json:"search_units"`
		} `json:"billed_units"`
	} `json:"meta"`
}

func makeSources(texts []string) []types.RerankSource {
	sources := make([]types.RerankSource, len(texts))
	for i, text := range texts {
		textCopy := text
		sources[i] = types.RerankSource{
			InlineDocumentSource: &types.RerankDocument{
				Type: types.RerankDocumentTypeText,
				TextDocument: &types.RerankTextDocument{
					Text: &textCopy,
				},
			},
			Type: types.RerankSourceTypeInline,
		}
	}
	return sources
}

func RerankText(client *bedrockagentruntime.Client, textQuery string, textSources []string, numResults int32, modelId string) (*bedrockagentruntime.RerankOutput, error) {
	modelPackageArn := fmt.Sprintf("arn:aws:bedrock:%s::foundation-model/%s", client.Options().Region, modelId)
	sources := makeSources(textSources)
	logger.Infof(context.TODO(), "modelPackageArn: %s", modelPackageArn)
	// Create input
	input := &bedrockagentruntime.RerankInput{
		Queries: []types.RerankQuery{
			{
				Type: "TEXT",
				TextQuery: &types.RerankTextDocument{
					Text: &textQuery,
				},
			},
		},
		Sources: sources,
		RerankingConfiguration: &types.RerankingConfiguration{
			Type: "BEDROCK_RERANKING_MODEL",
			BedrockRerankingConfiguration: &types.BedrockRerankingConfiguration{
				NumberOfResults: &numResults,
				ModelConfiguration: &types.BedrockRerankingModelConfiguration{
					ModelArn: &modelPackageArn,
				},
			},
		},
	}
	result, err := client.Rerank(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("error making rerank request: %v", err)
	}

	return result, nil
}
