package core

import (
	"reflect"
	"testing"
	"time"
)

func TestFullSystemPruneUsesAllVolumesAndForce(t *testing.T) {
	var got []string
	engine := NewEngine(t.TempDir(), "")
	engine.dockerExec = func(args ...string) (*ExecResult, error) {
		got = append([]string(nil), args...)
		return &ExecResult{}, nil
	}
	result := engine.Prune("system-all")
	if !result.Success {
		t.Fatalf("expected success, got %#v", result)
	}
	want := []string{"system", "prune", "--all", "--volumes", "-f"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("docker args = %#v, want %#v", got, want)
	}
}

func TestSystemPruneJobIsTrackedAsHostAction(t *testing.T) {
	engine := NewEngine(t.TempDir(), "")
	engine.dockerExec = func(args ...string) (*ExecResult, error) { return &ExecResult{}, nil }
	jobs := NewJobManager(nil)
	job, err := jobs.StartSystemPrune(engine)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		current, ok := jobs.Get(job.ID)
		if ok && current.Status != "running" {
			if current.Project != "(system)" || current.Action != "prune-system-all" || !current.Success {
				t.Fatalf("unexpected job: %#v", current)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("system prune job did not complete")
}
