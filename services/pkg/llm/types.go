package llm

type FileResponse struct {
	Object        string `json:"object"`
	ID            string `json:"id"`
	Purpose       string `json:"purpose"`
	Filename      string `json:"filename"`
	Bytes         int    `json:"bytes"`
	CreatedAt     int    `json:"created_at"`
	ExpiresAt     int    `json:"expires_at"`
	Status        string `json:"status"`
	StatusDetails string `json:"status_details"`
}

type CompletionsResponse struct {
	Choices []struct {
		FinishReason string      `json:"finish_reason"`
		Index        int64       `json:"index"`
		Logprobs     interface{} `json:"logprobs"`
		Message      struct {
			Annotations []interface{} `json:"annotations"`
			Content     string        `json:"content"`
			Refusal     interface{}   `json:"refusal"`
			Role        string        `json:"role"`
		} `json:"message"`
	} `json:"choices"`
	Created     int64  `json:"created"`
	ID          string `json:"id"`
	Model       string `json:"model"`
	Object      string `json:"object"`
	ServiceTier string `json:"service_tier"`
	Usage       struct {
		CompletionTokens        int64 `json:"completion_tokens"`
		CompletionTokensDetails struct {
			AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
			AudioTokens              int64 `json:"audio_tokens"`
			ReasoningTokens          int64 `json:"reasoning_tokens"`
			RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
		} `json:"completion_tokens_details"`
		PromptTokens        int64 `json:"prompt_tokens"`
		PromptTokensDetails struct {
			AudioTokens  int64 `json:"audio_tokens"`
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		TotalTokens int64 `json:"total_tokens"`
	} `json:"usage"`
}
