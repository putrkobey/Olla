package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type DockerModelRunnerResponse = profile.DockerModelRunnerResponse
type DockerModelRunnerModel = profile.DockerModelRunnerModel

// DockerModelRunnerConverter converts unified models to Docker Model Runner-compatible format
type DockerModelRunnerConverter struct {
	*BaseConverter
