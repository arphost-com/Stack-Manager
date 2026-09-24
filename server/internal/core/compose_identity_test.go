package core

import (
	"reflect"
	"testing"
)

func TestFindOrphanComposeServicesBlocksRenamedService(t *testing.T) {
	configured := []string{"npm", "database"}
	containers := "app|nginx-proxy-manager-app-1\ndatabase|nginx-proxy-manager-database-1\n"
	want := []string{`- service "app" (container "nginx-proxy-manager-app-1")`}
	if got := findOrphanComposeServices(configured, containers); !reflect.DeepEqual(got, want) {
		t.Fatalf("orphans = %#v, want %#v", got, want)
	}
}

func TestFindOrphanComposeServicesAllowsMatchingServices(t *testing.T) {
	configured := []string{"app", "database"}
	containers := "app|nginx-proxy-manager-app-1\ndatabase|nginx-proxy-manager-database-1\n"
	if got := findOrphanComposeServices(configured, containers); len(got) != 0 {
		t.Fatalf("orphans = %#v, want none", got)
	}
}
