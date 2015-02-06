package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// The SGLang and LMDeploy integration pages tell operators to copy the shipped
// profile to zz-<name>.yaml next to the original and set anthropic_support.enabled
// to false. The loader reads files in name order and the last profile with a given
