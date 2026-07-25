package core

import "os"

// HostHelperEnv returns the scrubbed environment used when the server invokes a
// privileged `docker run ... chroot /host` host helper (self-update, GPU setup,
// OS update, timezone, firewall). The environment is intentionally minimal, but
// it MUST carry DOCKER_HOST: the server reaches the daemon through the mounted
// /run directory (DOCKER_HOST=unix:///host-run/docker.sock), and the socket is
// NOT present at the default /var/run/docker.sock path inside the container.
//
// Historically these call sites hardcoded a PATH-only env, which worked only
// because the container used to bind-mount the socket file at its default path.
// After switching to the restart-resilient /run directory mount, dropping
// DOCKER_HOST made every helper fail with "Cannot connect to the Docker daemon
// at unix:///var/run/docker.sock". Always build the helper env through this
// function so that can't regress.
func HostHelperEnv() []string {
	env := []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	if dh := os.Getenv("DOCKER_HOST"); dh != "" {
		env = append(env, "DOCKER_HOST="+dh)
	}
	return env
}
