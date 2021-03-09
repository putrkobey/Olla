package handlers

import (
	"strconv"

	"github.com/putrkobey/Olla/internal/logger"
)

// rejectLogs samples the "Request rejected" WARN for requests Olla refuses to route (an
