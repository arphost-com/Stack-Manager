package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFillHostNetworkPortsPrefersEnvPorts(t *testing.T) {
	dir := t.TempDir()
	env := "ARPVPN_HTTP_PORT=8085\nARPVPN_HTTPS_PORT=\"8086\"\nARPVPN_UID=1000\nDATA_FOLDER=./data\nBAD_PORT=99999\n# OLD_PORT=1234\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0600); err != nil {
		t.Fatal(err)
	}
	p := &Project{Name: "arpvpn", Dir: dir, Containers: []Container{
		{ID: "a1", Name: "arpvpn-arpvpn-1", HostNetwork: true},
		{ID: "b2", Name: "arpvpn-arpvpn-init-1"},
	}}
	fillHostNetworkPorts(p, func(id string) []int {
		// The image still exposes its default ports, which this project's
		// .env has moved away from, as on docker02's arpvpn_staging.
		return []int{8085, 8086, 51820}
	})
	if want := []int{8085, 8086}; !reflect.DeepEqual(p.Containers[0].HostPorts, want) {
		t.Fatalf("host ports = %v, want %v", p.Containers[0].HostPorts, want)
	}
	if p.Containers[1].HostPorts != nil {
		t.Fatalf("bridge containers must not get host ports: %v", p.Containers[1].HostPorts)
	}
}

func TestParseExposedTCPPortsSkipsUDP(t *testing.T) {
	got := uniqueSortedPorts(parseExposedTCPPorts([]byte(`{"8123/tcp":{},"5353/udp":{},"80":{}}`)))
	if want := []int{80, 8123}; !reflect.DeepEqual(got, want) {
		t.Fatalf("exposed = %v, want %v", got, want)
	}
	if parseExposedTCPPorts([]byte("null")) != nil {
		t.Fatal("no exposed ports should give nil")
	}
}

func TestFillHostNetworkPortsFallsBackToExposedWithoutEnvPorts(t *testing.T) {
	p := &Project{Name: "home-assistant", Dir: t.TempDir(), Containers: []Container{
		{ID: "h1", Name: "home-assistant-1", HostNetwork: true},
	}}
	fillHostNetworkPorts(p, func(string) []int { return []int{8123} })
	if want := []int{8123}; !reflect.DeepEqual(p.Containers[0].HostPorts, want) {
		t.Fatalf("without .env ports the image EXPOSE list should be used, got %v", p.Containers[0].HostPorts)
	}
}

func TestFillHostNetworkPortsIgnoresStaleExposedPorts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("ARPVPN_HTTP_PORT=19085\nARPVPN_HTTPS_PORT=19086\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p := &Project{Name: "arpvpn", Dir: dir, Containers: []Container{{ID: "a1", HostNetwork: true}}}
	fillHostNetworkPorts(p, func(string) []int { return []int{8085, 8086} })
	if want := []int{19085, 19086}; !reflect.DeepEqual(p.Containers[0].HostPorts, want) {
		t.Fatalf(".env ports must replace the image defaults, got %v", p.Containers[0].HostPorts)
	}
}
