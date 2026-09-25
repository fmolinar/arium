package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// healthGrace is how long past its scheduled time a run may take before the
// scheduler counts as stuck. Sources run concurrently under a per-source
// timeout, so a healthy run finishes well within this.
const healthGrace = 10 * time.Minute

// heartbeat is written by the scheduler each time it starts waiting for the
// next run. -healthcheck reads it back.
type heartbeat struct {
	NextRun time.Time `json:"next_run"`
}

// writeHeartbeat atomically records the next scheduled run time at path.
func writeHeartbeat(path string, next time.Time) error {
	data, err := json.Marshal(heartbeat{NextRun: next})
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-heartbeat-*")
	if err != nil {
		return fmt.Errorf("write heartbeat: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write heartbeat: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write heartbeat: %w", err)
	}

	return os.Rename(tmp.Name(), path)
}

// checkHeartbeat reports an error if the heartbeat at path is missing or if
// its run is more than healthGrace overdue, meaning the scheduler loop died
// or a run is hung.
func checkHeartbeat(path string, now time.Time) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read heartbeat: %w", err)
	}

	var hb heartbeat
	if err := json.Unmarshal(data, &hb); err != nil {
		return fmt.Errorf("parse heartbeat %s: %w", path, err)
	}

	if overdue := now.Sub(hb.NextRun); overdue > healthGrace {
		return fmt.Errorf("run scheduled for %s is %s overdue", hb.NextRun.Format(time.RFC3339), overdue.Round(time.Second))
	}

	return nil
}
