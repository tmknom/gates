package handler

import (
	"errors"
	"testing"
)

func TestNewRequestID_InvalidUTF8ReturnsErrInvalidRequestID(t *testing.T) {
	if _, err := NewRequestID("\xff"); !errors.Is(err, ErrInvalidRequestID) {
		t.Errorf("NewRequestID error = %v, want %v", err, ErrInvalidRequestID)
	}
}
