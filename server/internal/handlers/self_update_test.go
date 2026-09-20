package handlers

import "testing"

func TestSelfUpdateProgressError(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		runningVersion string
		running        bool
		wantError      bool
	}{
		{name: "running", output: "self-update starting", runningVersion: "1.5.5+old", running: true},
		{name: "verified", output: "self-update starting\nrebuilding at abc1234\nself-update done", runningVersion: "1.5.7+abc1234"},
		{name: "version mismatch", output: "self-update starting\nrebuilding at abc1234\nself-update done", runningVersion: "1.5.5+old", wantError: true},
		{name: "command failure", output: "self-update starting\nERROR: compose up failed", runningVersion: "1.5.5+old", wantError: true},
		{name: "incomplete", output: "self-update starting\nrebuilding at abc1234", runningVersion: "1.5.7+abc1234", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := selfUpdateProgressError(test.output, test.runningVersion, test.running)
			if (got != "") != test.wantError {
				t.Fatalf("error=%q wantError=%v", got, test.wantError)
			}
		})
	}
}
