package core

import "testing"

func TestAggregateProjectState(t *testing.T) {
	tests := []struct {
		name       string
		containers []Container
		want       string
	}{
		{name: "no containers", want: "stopped"},
		{name: "running", containers: []Container{{State: "running"}}, want: "running"},
		{name: "restart loop wins over running", containers: []Container{{State: "running"}, {State: "restarting"}}, want: "restarting"},
		{name: "completed init does not hide running app", containers: []Container{{State: "exited"}, {State: "running"}}, want: "running"},
		{name: "paused wins over running", containers: []Container{{State: "running"}, {State: "paused"}}, want: "paused"},
		{name: "all exited", containers: []Container{{State: "exited"}, {State: "exited"}}, want: "exited"},
		{name: "unknown Docker state", containers: []Container{{State: "mystery"}}, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregateProjectState(tt.containers); got != tt.want {
				t.Fatalf("aggregateProjectState() = %q, want %q", got, tt.want)
			}
		})
	}
}
