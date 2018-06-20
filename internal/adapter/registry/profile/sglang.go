package profile

// SGLangResponse represents the response structure from SGLang /v1/models endpoint
type SGLangResponse struct {
	Object string        `json:"object"`
	Data   []SGLangModel `json:"data"`
}

