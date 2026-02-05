package plugin

import (
	"testing"
)

func TestRegisterPluginJobs(t *testing.T) {
	t.Run("Nil manager returns 0", func(t *testing.T) {
		count := RegisterPluginJobs(nil, nil)
		if count != 0 {
			t.Errorf("expected 0, got %d", count)
		}
	})
	
	t.Run("Nil scheduler returns 0", func(t *testing.T) {
		mgr := NewManager(nil)
		count := RegisterPluginJobs(mgr, nil)
		if count != 0 {
			t.Errorf("expected 0, got %d", count)
		}
	})
}

func TestJobSpec(t *testing.T) {
	// Test JobSpec struct
	job := JobSpec{
		ID:          "test-job",
		Schedule:    "0 * * * *",
		Handler:     "my_handler",
		Description: "Test job",
		Enabled:     true,
		Timeout:     "5m",
	}
	
	if job.ID != "test-job" {
		t.Errorf("expected test-job, got %s", job.ID)
	}
	if job.Schedule != "0 * * * *" {
		t.Errorf("expected 0 * * * *, got %s", job.Schedule)
	}
	if !job.Enabled {
		t.Error("expected enabled")
	}
}
