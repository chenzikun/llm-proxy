package aws

type CohereRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	TopN      int      `json:"top_n"`
	Documents []string `json:"documents"`
}

type CohereEmbedRequest struct {
	InputType      string   `json:"input_type"`
	EmbeddingTypes []string `json:"embedding_types"`
	Texts          []string `json:"texts,omitempty"`
	Images         []string `json:"images,omitempty"`
}

type TitanEmbedRequest struct {
	InputText  string `json:"inputText"`
	Dimensions int    `json:"dimensions,omitempty"`
	Normalize  *bool  `json:"normalize,omitempty"`
}

type TitanEmbedResponse struct {
	Embedding           []float64 `json:"embedding"`
	InputTextTokenCount int       `json:"inputTextTokenCount"`
}

type CohereEmbedResponse struct {
	Id         string                     `json:"id"`
	Embeddings map[string][][]interface{} `json:"embeddings"`
	Texts      []interface{}              `json:"texts"`
	Images     []struct {
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Format   string `json:"format"`
		BitDepth int    `json:"bit_depth"`
	} `json:"images"`
	Meta struct {
		ApiVersion struct {
			Version string `json:"version"`
		} `json:"api_version"`
		BilledUnits struct {
			Images int `json:"images"`
		} `json:"billed_units"`
	} `json:"meta"`
}
