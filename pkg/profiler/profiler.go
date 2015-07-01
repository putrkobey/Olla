package profiler

import (
	"log"
	"net/http"
	"net/http/pprof"
	"time"
)

// InitialiseProfiler sets up the HTTP server for pprof profiling.
// Based off  https://github.com/putrkobey/smash/blob/main/pkg/profiler/profiler.go
func InitialiseProfiler(address string) {
	http.DefaultServeMux = http.NewServeMux()
	go func() {
