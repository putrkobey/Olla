package ports

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type mockSecurityValidator struct {
	name          string
	shouldAllow   bool
	errorToReturn error
	callCount     int
}

func (m *mockSecurityValidator) Name() string {
	return m.name
}

func (m *mockSecurityValidator) Validate(ctx context.Context, req SecurityRequest) (SecurityResult, error) {
	m.callCount++

	if m.errorToReturn != nil {
		return SecurityResult{}, m.errorToReturn
	}

	if !m.shouldAllow {
		return SecurityResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Mock validator %s rejected request", m.name),
		}, nil
	}

	return SecurityResult{
		Allowed: true,
	}, nil
}

func TestNewSecurityChain(t *testing.T) {
	validator1 := &mockSecurityValidator{name: "validator1", shouldAllow: true}
	validator2 := &mockSecurityValidator{name: "validator2", shouldAllow: true}

	chain := NewSecurityChain(validator1, validator2)

	if chain == nil {
		t.Fatal("NewSecurityChain returned nil")
	}

	validators := chain.GetValidators()
	if len(validators) != 2 {
		t.Errorf("Expected 2 validators, got %d", len(validators))
	}

	if validators[0].Name() != "validator1" {
		t.Errorf("Expected first validator name 'validator1', got %q", validators[0].Name())
	}

	if validators[1].Name() != "validator2" {
		t.Errorf("Expected second validator name 'validator2', got %q", validators[1].Name())
	}
}

func TestSecurityChain_Validate_AllAllow(t *testing.T) {
	validator1 := &mockSecurityValidator{name: "validator1", shouldAllow: true}
	validator2 := &mockSecurityValidator{name: "validator2", shouldAllow: true}
	validator3 := &mockSecurityValidator{name: "validator3", shouldAllow: true}

	chain := NewSecurityChain(validator1, validator2, validator3)
	ctx := context.Background()

	req := SecurityRequest{
		ClientID: "192.168.1.100",
		Endpoint: "/api/test",
		Method:   "POST",
	}

	result, err := chain.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Chain validation failed: %v", err)
	}

	if !result.Allowed {
		t.Errorf("Expected request to be allowed, got: %s", result.Reason)
	}

	if validator1.callCount != 1 {
		t.Errorf("Expected validator1 to be called once, got %d", validator1.callCount)
	}
	if validator2.callCount != 1 {
		t.Errorf("Expected validator2 to be called once, got %d", validator2.callCount)
	}
	if validator3.callCount != 1 {
		t.Errorf("Expected validator3 to be called once, got %d", validator3.callCount)
	}
}

func TestSecurityChain_Validate_FirstRejects(t *testing.T) {
	validator1 := &mockSecurityValidator{name: "validator1", shouldAllow: false}
	validator2 := &mockSecurityValidator{name: "validator2", shouldAllow: true}
	validator3 := &mockSecurityValidator{name: "validator3", shouldAllow: true}

	chain := NewSecurityChain(validator1, validator2, validator3)
	ctx := context.Background()

	req := SecurityRequest{
		ClientID: "192.168.1.100",
		Endpoint: "/api/test",
		Method:   "POST",
	}

	result, err := chain.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Chain validation failed: %v", err)
	}

	if result.Allowed {
		t.Error("Expected request to be rejected by first validator")
	}

	if result.Reason != "Mock validator validator1 rejected request" {
		t.Errorf("Expected specific rejection reason, got: %s", result.Reason)
	}

	if validator1.callCount != 1 {
		t.Errorf("Expected validator1 to be called once, got %d", validator1.callCount)
	}
	if validator2.callCount != 0 {
		t.Errorf("Expected validator2 to not be called, got %d calls", validator2.callCount)
	}
	if validator3.callCount != 0 {
		t.Errorf("Expected validator3 to not be called, got %d calls", validator3.callCount)
	}
}

func TestSecurityChain_Validate_MiddleRejects(t *testing.T) {
	validator1 := &mockSecurityValidator{name: "validator1", shouldAllow: true}
	validator2 := &mockSecurityValidator{name: "validator2", shouldAllow: false}
	validator3 := &mockSecurityValidator{name: "validator3", shouldAllow: true}

	chain := NewSecurityChain(validator1, validator2, validator3)
	ctx := context.Background()

	req := SecurityRequest{
		ClientID: "192.168.1.100",
		Endpoint: "/api/test",
		Method:   "POST",
	}

	result, err := chain.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Chain validation failed: %v", err)
	}

	if result.Allowed {
		t.Error("Expected request to be rejected by middle validator")
	}

	if result.Reason != "Mock validator validator2 rejected request" {
		t.Errorf("Expected specific rejection reason, got: %s", result.Reason)
	}

	if validator1.callCount != 1 {
		t.Errorf("Expected validator1 to be called once, got %d", validator1.callCount)
	}
	if validator2.callCount != 1 {
		t.Errorf("Expected validator2 to be called once, got %d", validator2.callCount)
	}
	if validator3.callCount != 0 {
		t.Errorf("Expected validator3 to not be called, got %d calls", validator3.callCount)
	}
}

func TestSecurityChain_Validate_ErrorHandling(t *testing.T) {
	expectedError := errors.New("validator error")
	validator1 := &mockSecurityValidator{name: "validator1", shouldAllow: true}
	validator2 := &mockSecurityValidator{name: "validator2", errorToReturn: expectedError}
	validator3 := &mockSecurityValidator{name: "validator3", shouldAllow: true}

	chain := NewSecurityChain(validator1, validator2, validator3)
	ctx := context.Background()

	req := SecurityRequest{
		ClientID: "192.168.1.100",
		Endpoint: "/api/test",
		Method:   "POST",
	}

	result, err := chain.Validate(ctx, req)
	if err == nil {
		t.Fatal("Expected error to be returned")
	}

	if err.Error() != "validator error" {
		t.Errorf("Expected specific error, got: %v", err)
	}

	if validator1.callCount != 1 {
		t.Errorf("Expected validator1 to be called once, got %d", validator1.callCount)
	}
	if validator2.callCount != 1 {
		t.Errorf("Expected validator2 to be called once, got %d", validator2.callCount)
	}
	if validator3.callCount != 0 {
		t.Errorf("Expected validator3 to not be called, got %d calls", validator3.callCount)
	}

	if result.Allowed {
		t.Error("Expected result.Allowed to be false on error")
	}
}

func TestSecurityChain_Validate_EmptyChain(t *testing.T) {
	chain := NewSecurityChain()
	ctx := context.Background()

	req := SecurityRequest{
		ClientID: "192.168.1.100",
		Endpoint: "/api/test",
		Method:   "POST",
	}

	result, err := chain.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Empty chain validation failed: %v", err)
	}

	if !result.Allowed {
		t.Error("Expected empty chain to allow request")
	}
}

