package core

import "testing"

func TestProjectContainsContainerID(t *testing.T) {
	project := Project{Containers: []Container{{ID: "3ba234e856f1"}}}
	if !projectContainsContainerID(project, "3ba234e856f1aabbcc") {
		t.Fatal("expected full hostname ID to match discovered short container ID")
	}
	if projectContainsContainerID(project, "docker02") {
		t.Fatal("host names must not be mistaken for container IDs")
	}
}

func TestIsControllerProjectNameFallback(t *testing.T) {
	t.Setenv("STACK_MANAGER_PROJECT_NAME", "")
	engine := NewEngine("/docker", "")
	if !engine.IsControllerProject(&Project{Name: "stack-manager"}) {
		t.Fatal("expected conventional Stack Manager project name to be protected")
	}
	if engine.IsControllerProject(&Project{Name: "webtop"}) {
		t.Fatal("unrelated project must not be protected")
	}
}

func TestIsControllerProjectConfiguredName(t *testing.T) {
	t.Setenv("STACK_MANAGER_PROJECT_NAME", "controller-renamed")
	engine := NewEngine("/docker", "")
	if !engine.IsControllerProject(&Project{Name: "controller-renamed"}) {
		t.Fatal("expected configured controller name to be protected")
	}
	if !engine.IsControllerProject(&Project{Name: "stack-manager"}) {
		t.Fatal("conventional controller name must remain protected as a fail-safe")
	}
}

func TestControllerPullIsRejectedInsideEngine(t *testing.T) {
	t.Setenv("STACK_MANAGER_PROJECT_NAME", "")
	engine := NewEngine("/docker", "")
	result := engine.Pull(&Project{Name: "stack-manager"}, 0)
	if result.Success || result.ExitCode != -1 || result.Output != ControllerImageActionMessage+"\n" {
		t.Fatalf("unexpected protected pull result: %#v", result)
	}
}
