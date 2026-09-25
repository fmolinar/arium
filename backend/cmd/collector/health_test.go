package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHeartbeat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heartbeat.json")
	next := time.Date(2026, 9, 24, 16, 0, 0, 0, time.UTC)

	if err := checkHeartbeat(path, next); err == nil {
		t.Error("missing heartbeat: want error, got nil")
	}

	if err := writeHeartbeat(path, next); err != nil {
		t.Fatalf("writeHeartbeat: %v", err)
	}

	tests := []struct {
		name    string
		now     time.Time
		wantErr bool
	}{
		{"waiting for run", next.Add(-8 * time.Hour), false},
		{"run in progress", next.Add(healthGrace), false},
		{"run overdue", next.Add(healthGrace + time.Second), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkHeartbeat(path, tt.now)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkHeartbeat = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
