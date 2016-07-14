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
