import test from 'node:test';
import assert from 'node:assert/strict';

import { containerPortsString } from './hostPorts.js';

test('bridge containers keep their Docker port text', () => {
  assert.equal(containerPortsString({ ports: '0.0.0.0:8080->80/tcp' }), '0.0.0.0:8080->80/tcp');
});

test('host network containers use their declared ports', () => {
  assert.equal(
    containerPortsString({ host_network: true, host_ports: [8085, 8086] }),
    '0.0.0.0:8085->8085/tcp, 0.0.0.0:8086->8086/tcp',
  );
});

test('missing or invalid host ports give an empty string', () => {
  assert.equal(containerPortsString({ host_network: true }), '');
  assert.equal(containerPortsString({ host_network: true, host_ports: [0, 70000, 'x'] }), '');
  assert.equal(containerPortsString(null), '');
});
