import assert from 'node:assert/strict';
import { execFileSync, spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import net from 'node:net';
import { tmpdir } from 'node:os';
import path from 'node:path';
import process from 'node:process';
import { setTimeout as sleep } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

import { VelarixClient } from '../src/client.ts';

async function waitForHealth(url: string): Promise<void> {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    try {
      const res = await fetch(`${url}/health`);
      if (res.ok) return;
    } catch {
      // retry
    }
    await sleep(500);
  }
  throw new Error('server did not become healthy');
}

async function canListenLocally(): Promise<boolean> {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once('error', () => resolve(false));
    server.listen(0, '127.0.0.1', () => {
      server.close(() => resolve(true));
    });
  });
}

function startServer(): ChildProcess {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const repoRoot = path.resolve(here, '../../..');
  const gocache = mkdtempSync(path.join(tmpdir(), 'velarix-go-cache-'));
  const binary = path.join(mkdtempSync(path.join(tmpdir(), 'velarix-sdk-bin-')), 'velarix-test-bin');
  const env = {
    ...process.env,
    VELARIX_ENCRYPTION_KEY: 'test_32_byte_secure_key_12345678',
    VELARIX_API_KEY: 'test_key',
    VELARIX_ENV: 'dev',
    PORT: '8090',
    VELARIX_BADGER_PATH: mkdtempSync(path.join(tmpdir(), 'velarix-sdk-ts-')),
    GOCACHE: gocache,
  };
  execFileSync('go', ['build', '-o', binary, 'main.go'], {
    cwd: repoRoot,
    env,
    stdio: 'pipe',
  });
  return spawn(binary, [], {
    cwd: repoRoot,
    env,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

async function main(): Promise<void> {
  if (!(await canListenLocally())) {
    // eslint-disable-next-line no-console
    console.warn('Skipping TypeScript SDK integration test: local TCP listen is not permitted in this environment.');
    return;
  }

  const server = startServer();
  let stderr = '';
  let stdout = '';
  let exitSummary = 'server still running';
  server.stdout?.on('data', (chunk) => {
    stdout += String(chunk);
  });
  server.stderr?.on('data', (chunk) => {
    stderr += String(chunk);
  });
  server.on('exit', (code, signal) => {
    exitSummary = `exit code=${code} signal=${signal}`;
  });
  try {
    try {
      await waitForHealth('http://localhost:8090');
    } catch (err) {
      throw new Error(`${String(err)}\nserver status: ${exitSummary}\nserver stdout:\n${stdout}\nserver stderr:\n${stderr}`);
    }

    const client = new VelarixClient({ baseUrl: 'http://localhost:8090', apiKey: 'test_key' });
    const session = client.session('ts_sdk_decision_sess');

    await session.observe('ts_root', { approved_by: 'bot' });
    await session.derive('ts_decision_fact', [['ts_root']], { summary: 'typescript derived approval' });
    const decision = await session.createDecision('ts_approval', {
      factId: 'ts_decision_fact',
      subjectRef: 'invoice-ts-1',
      targetRef: 'vendor-ts-1',
      dependencyFactIds: ['ts_root'],
    });
    assert.equal(decision.decision_type, 'ts_approval');

    const activeCheck = await session.executeCheck(decision.decision_id);
    assert.equal(activeCheck.executable, true);
    assert.equal(typeof activeCheck.execution_token, 'string');

    const listed = await session.listDecisions();
    assert.equal(listed.some((item) => item.decision_id === decision.decision_id), true);

    await session.invalidate('ts_root');

    const blockedCheck = await session.executeCheck(decision.decision_id);
    assert.equal(blockedCheck.executable, false);
    assert.equal(blockedCheck.reason_codes.includes('dependency_missing_or_invalid') || blockedCheck.reason_codes.includes('dependency_invalid'), true);

    const whyBlocked = await session.getDecisionWhyBlocked(decision.decision_id);
    assert.equal(whyBlocked.decision.decision_id, decision.decision_id);
    assert.equal(Array.isArray(whyBlocked.blocked_by), true);
  } finally {
    server.kill('SIGTERM');
    await sleep(500);
  }
}

main().catch((err) => {
  // eslint-disable-next-line no-console
  console.error(err);
  process.exitCode = 1;
});
