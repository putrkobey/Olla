package translator

import (
	"errors"
	"fmt"

	"github.com/tidwall/gjson"
)

// ExtractModelName performs a lightweight extraction of the top-level "model"
// field from a JSON request body. This exists to avoid a full unmarshal on the
// hot path -- the handler needs the model name for endpoint filtering and
// routing decisions before it knows whether passthrough or translation will be
// used. A full TransformRequest parse is deferred to the translation path only.
//
// Uses gjson.GetBytes which scans forward to the first matching key without
