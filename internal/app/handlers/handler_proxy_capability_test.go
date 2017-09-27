package handlers

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func TestFilterEndpointsByCapabilities(t *testing.T) {
	// Create mock logger
	styledLog := &mockStyledLogger{}

	// Create test endpoints
	endpoint1, _ := url.Parse("http://localhost:11434")
	endpoint2, _ := url.Parse("http://localhost:11435")
	endpoint3, _ := url.Parse("http://localhost:11436")
	endpoint4, _ := url.Parse("http://localhost:11437")

	endpoints := []*domain.Endpoint{
		{
			Name:      "endpoint1",
			URL:       endpoint1,
			URLString: "http://localhost:11434",
			Type:      domain.ProfileOllama,
		},
		{
			Name:      "endpoint2",
			URL:       endpoint2,
			URLString: "http://localhost:11435",
			Type:      domain.ProfileOllama,
		},
		{
			Name:      "endpoint3",
			URL:       endpoint3,
			URLString: "http://localhost:11436",
			Type:      domain.ProfileOpenAICompatible,
		},
		{
			Name:      "endpoint4",
			URL:       endpoint4,
			URLString: "http://localhost:11437",
			Type:      domain.ProfileOpenAICompatible,
		},
	}

	// Mock model registry with capability support
	modelRegistry := &mockCapabilityModelRegistry{
		modelsByCapability: map[string][]*domain.UnifiedModel{
			"vision": {
				{
					ID:           "llava:13b",
					Capabilities: []string{"vision", "chat", "streaming"},
					SourceEndpoints: []domain.SourceEndpoint{
						{EndpointURL: "http://localhost:11434", LastSeen: time.Now()},
						{EndpointURL: "http://localhost:11435", LastSeen: time.Now()},
					},
				},
				{
					ID:           "gpt-4-vision",
					Capabilities: []string{"vision", "chat", "function_calling"},
					SourceEndpoints: []domain.SourceEndpoint{
						{EndpointURL: "http://localhost:11436", LastSeen: time.Now()},
					},
				},
			},
			"function_calling": {
				{
					ID:           "gpt-4-turbo",
					Capabilities: []string{"chat", "function_calling", "streaming"},
					SourceEndpoints: []domain.SourceEndpoint{
						{EndpointURL: "http://localhost:11436", LastSeen: time.Now()},
						{EndpointURL: "http://localhost:11437", LastSeen: time.Now()},
					},
				},
				{
					ID:           "gpt-4-vision",
					Capabilities: []string{"vision", "chat", "function_calling"},
					SourceEndpoints: []domain.SourceEndpoint{
						{EndpointURL: "http://localhost:11436", LastSeen: time.Now()},
					},
				},
			},
			"tools": {
				{
					ID:           "gpt-4-turbo",
					Capabilities: []string{"chat", "function_calling", "streaming"},
					SourceEndpoints: []domain.SourceEndpoint{
						{EndpointURL: "http://localhost:11436", LastSeen: time.Now()},
						{EndpointURL: "http://localhost:11437", LastSeen: time.Now()},
					},
				},
				{
					ID:           "gpt-4-vision",
					Capabilities: []string{"vision", "chat", "function_calling"},
					SourceEndpoints: []domain.SourceEndpoint{
						{EndpointURL: "http://localhost:11436", LastSeen: time.Now()},
					},
				},
			},
			"embeddings": {
				{
					ID:           "text-embedding-ada-002",
					Capabilities: []string{"embeddings"},
