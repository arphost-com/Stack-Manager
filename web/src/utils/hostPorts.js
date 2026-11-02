// containerPortsString returns the port text the UI should use for a container.
// Docker reports no port mappings for network_mode: host containers, so the
// server sends the ports the project declares in host_ports instead. Those are
// rendered as host-published mappings so the UI link, the firewall action and
// Add to proxy work the same way they do for bridge containers.
export function containerPortsString(container) {
  if (!container) return '';
  if (container.ports) return container.ports;
  if (!container.host_network || !Array.isArray(container.host_ports)) return '';
  return container.host_ports
    .filter((port) => Number.isInteger(port) && port > 0 && port <= 65535)
    .map((port) => `0.0.0.0:${port}->${port}/tcp`)
    .join(', ');
}
