package asynqadp

import "testing"

func TestNewWorker_InvalidURL_Error(t *testing.T) {
	if _, err := NewWorker("://bad", nil, nil, nil, nil, nil, false, false); err == nil {
		t.Fatalf("expected error")
	}
}
