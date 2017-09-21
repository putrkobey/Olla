package profile

/*
	Lemonade profile for AMD's Lemonade model registry.

	Reference (02/10/2025):
	- Lemonade API documentation: https://lemonade-server.ai/docs/server/server_spec/#start-the-rest-api-server

*/
// LemonadeResponse represents the response structure from Lemonade /api/v1/models endpoint
type LemonadeResponse struct {
	Object string          `json:"object"`
	Data   []LemonadeModel `json:"data"`
}

// LemonadeModel represents a model in Lemonade response with Lemonade-specific metadata
