import assert from 'node:assert/strict';
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import process from 'node:process';
import { setTimeout as sleep } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

import { VelarixClient } from '../src/client.ts';

async function waitForHealth(url: string): Promise<void> {
  for (let attempt = 0; attempt < 30; attempt += 1) {
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

function startServer(): ChildProcess {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const repoRoot = path.resolve(here, '../../..');
  const env = {
    ...process.env,
    VELARIX_ENCRYPTION_KEY: 'test_32_byte_secure_key_12345678',
    VELARIX_API_KEY: 'test_key',
    VELARIX_ENV: 'dev',
    PORT: '8090',
    VELARIX_BADGER_PATH: mkdtempSync(path.join(tmpdir(), 'velarix-sdk-ts-')),
  };
  return spawn('go', ['run', 'main.go'], {
    cwd: repoRoot,
    env,
    stdio: 'ignore',
  });
}

async function main(): Promise<void> {
  const server = startServer();
  try {
    await waitForHealth('http://localhost:8090');

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
