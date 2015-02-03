package inspector

import "fmt"

type InspectionError struct {
	Err           error
	InspectorName string
	Path          string
}

func (e *InspectionError) Error() string {
	return fmt.Sprintf("inspection failed for inspector '%s' on path '%s': %v", e.InspectorName, e.Path, e.Err)
}

func (e *InspectionError) Unwrap() error {
	return e.Err
}

func NewInspectionError(inspectorName, path string, err error) *InspectionError {
	return &InspectionError{
