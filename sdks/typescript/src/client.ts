import { EventSourcePolyfill } from 'event-source-polyfill';
import type { Fact, ExplanationNode, ChangeEvent, JournalEntry, DecisionRecordPayload } from './types.js';

function sleep(ms: number): Promise<void> {
  if (ms <= 0) return Promise.resolve();
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function retryAfterMs(h: string | null): number | null {
  if (!h) return null;
  const s = h.trim();
  if (!s) return null;
  const seconds = Number(s);
  if (!Number.isNaN(seconds) && Number.isFinite(seconds) && seconds >= 0) return Math.min(30_000, seconds * 1000);
  const d = new Date(s);
  const ms = d.getTime() - Date.now();
  if (Number.isFinite(ms)) return Math.max(0, Math.min(30_000, ms));
  return null;
}

export class VelarixSession {
  private client: VelarixClient;
  private sessionId: string;
  private baseUrl: string;

  constructor(client: VelarixClient, sessionId: string) {
    this.client = client;
    this.sessionId = sessionId;
    this.baseUrl = `${client.getBaseUrl()}/v1/s/${sessionId}`;
  }

  private getHeaders() {
    return this.client.getHeaders();
  }

  private idemKey(explicit?: string): string {
    if (explicit) return explicit;
    // Prefer standards-based UUID when available
    const g: any = globalThis as any;
    if (g.crypto && typeof g.crypto.randomUUID === 'function') return g.crypto.randomUUID();
    return `idem_${Date.now()}_${Math.random().toString(16).slice(2)}`;
  }

  async observe(factId: string, payload?: Record<string, any>, idempotencyKey?: string): Promise<Fact> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Content-Type': 'application/json', 'Idempotency-Key': this.idemKey(idempotencyKey) },
      body: JSON.stringify({
        id: factId,
        is_root: true,
        manual_status: 1,
        payload: payload || {}
      })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async derive(factId: string, justifications: string[][], payload?: Record<string, any>, idempotencyKey?: string): Promise<Fact> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Content-Type': 'application/json', 'Idempotency-Key': this.idemKey(idempotencyKey) },
      body: JSON.stringify({
        id: factId,
        is_root: false,
        justification_sets: justifications,
        payload: payload || {}
      })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async invalidate(factId: string, idempotencyKey?: string): Promise<void> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/facts/${factId}/invalidate`, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Idempotency-Key': this.idemKey(idempotencyKey) }
    });
    if (!res.ok) throw new Error(await res.text());
  }

  async getSlice(format: 'json' | 'markdown' = 'json', maxFacts: number = 50): Promise<Fact[] | string> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/slice?format=${format}&max_facts=${maxFacts}`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return format === 'markdown' ? res.text() : res.json();
  }

  async getFact(factId: string): Promise<Fact> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/facts/${factId}`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getWhy(factId: string): Promise<ExplanationNode[]> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/facts/${factId}/why`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getImpact(factId: string): Promise<any> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/facts/${factId}/impact`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getHistory(): Promise<JournalEntry[]> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/history`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async revalidate(idempotencyKey?: string): Promise<any> {
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/revalidate`, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Idempotency-Key': this.idemKey(idempotencyKey) }
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async recordDecision(payload: DecisionRecordPayload, idempotencyKey?: string): Promise<JournalEntry> {
    if (!payload || !payload.kind) throw new Error('recordDecision requires payload.kind');
    const res = await this.client.fetchWithRetry(`${this.baseUrl}/history`, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Content-Type': 'application/json', 'Idempotency-Key': this.idemKey(idempotencyKey) },
      body: JSON.stringify({
        type: 'decision_record',
        payload
      })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  listen(onEvent: (event: ChangeEvent) => void): () => void {
    const es = new EventSourcePolyfill(`${this.baseUrl}/events`, {
      headers: this.getHeaders()
    });
    es.onmessage = (event: any) => {
      if (event.data) {
        onEvent(JSON.parse(event.data));
      }
    };
    return () => es.close();
  }
}

export interface VelarixConfig {
  baseUrl?: string;
  apiKey?: string | null;
  maxRetries?: number;
}

export class VelarixClient {
  private baseUrl: string;
  private apiKey: string | null;
  private maxRetries: number;

  constructor(config: VelarixConfig = {}) {
    this.baseUrl = (config.baseUrl || 'http://localhost:8080').replace(/\/$/, '');
    this.apiKey = config.apiKey || null;
    this.maxRetries = typeof config.maxRetries === 'number' && config.maxRetries >= 0 ? Math.floor(config.maxRetries) : 4;
  }

  getBaseUrl(): string {
    return this.baseUrl;
  }

  getHeaders(): Record<string, string> {
    return this.apiKey ? { 'Authorization': `Bearer ${this.apiKey}` } : {};
  }

  async fetchWithRetry(url: string, init: RequestInit): Promise<Response> {
    const retryable = new Set([429, 502, 503, 504]);
    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      let res: Response;
      try {
        res = await fetch(url, init);
      } catch (e) {
        if (attempt >= this.maxRetries) throw e;
        const base = 200 * Math.pow(2, attempt);
        const jitter = 0.8 + Math.random() * 0.4;
        await sleep(Math.min(3_000, Math.round(base * jitter)));
        continue;
      }
      if (res.ok) return res;
      if (!retryable.has(res.status) || attempt >= this.maxRetries) return res;
      const ra = retryAfterMs(res.headers.get('Retry-After'));
      const base = 250 * Math.pow(2, attempt);
      const jitter = 0.85 + Math.random() * 0.3;
      await sleep(ra ?? Math.min(5_000, Math.round(base * jitter)));
    }
    // unreachable
    // eslint-disable-next-line @typescript-eslint/no-throw-literal
    throw new Error('unreachable');
  }

  session(sessionId: string): VelarixSession {
    return new VelarixSession(this, sessionId);
  }

  async getSessions(): Promise<any[]> {
    const res = await this.fetchWithRetry(`${this.baseUrl}/v1/sessions`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getUsage(): Promise<Record<string, number>> {
    const res = await this.fetchWithRetry(`${this.baseUrl}/v1/org/usage`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }
}
