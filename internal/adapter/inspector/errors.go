package inspector

import "fmt"

type InspectionError struct {
	Err           error
	InspectorName string
	Path          string
}

