package metrics

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

var ollamaResponse = []byte(`{
	"model": "llama2",
	"done": true,
	"context": [1, 2, 3],
	"total_duration": 5589157167,
	"load_duration": 3013701500,
	"prompt_eval_count": 26,
	"prompt_eval_duration": 2000000000,
	"eval_count": 290,
	"eval_duration": 2575455000
}`)
