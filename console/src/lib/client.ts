import { EventSourcePolyfill } from 'event-source-polyfill';
import type {
  APIKeyRecord,
  ChangeEvent,
  ExplanationNode,
  Fact,
  ImpactReport,
  JournalEntry,
  SessionInfo,
} from './types';

export class VelarixSession {
  private readonly baseUrl: string;
  private readonly client: VelarixClient;

  constructor(client: VelarixClient, sessionId: string) {
    this.client = client;
    this.baseUrl = `${client.getBaseUrl()}/s/${sessionId}`;
  }

  private headers(): Record<string, string> {
    return this.client.buildHeaders();
  }

  async observe(factId: string, payload?: Record<string, unknown>): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: this.headers(),
      body: JSON.stringify({
        id: factId,
        is_root: true,
        manual_status: 1,
        payload: payload ?? {},
      }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async derive(factId: string, justifications: string[][], payload?: Record<string, unknown>): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: this.headers(),
      body: JSON.stringify({
        id: factId,
        is_root: false,
        justification_sets: justifications,
        payload: payload ?? {},
      }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async invalidate(factId: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/invalidate`, {
      method: 'POST',
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(await res.text());
  }

  async setConfig(schema?: string, mode?: 'strict' | 'warn'): Promise<{ schema: string; enforcement_mode: string }> {
    const res = await fetch(`${this.baseUrl}/config`, {
      method: 'POST',
      headers: this.headers(),
      body: JSON.stringify({ schema, enforcement_mode: mode }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getSlice(format: 'json' | 'markdown' = 'json', maxFacts = 50): Promise<unknown> {
    const res = await fetch(`${this.baseUrl}/slice?format=${format}&max_facts=${maxFacts}`, {
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(await res.text());
    return format === 'markdown' ? res.text() : res.json();
  }

  async getFacts(validOnly = false): Promise<Fact[]> {
    const res = await fetch(`${this.baseUrl}/facts${validOnly ? '?valid=true' : ''}`, {
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getFact(factId: string): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}`, {
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getWhy(factId: string): Promise<ExplanationNode[]> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/why`, {
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getImpact(factId: string): Promise<ImpactReport> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/impact`, {
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getHistory(): Promise<JournalEntry[]> {
    const res = await fetch(`${this.baseUrl}/history`, {
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  listen(onEvent: (event: ChangeEvent) => void): () => void {
    const es = new EventSourcePolyfill(`${this.baseUrl}/events`, {
      headers: this.headers(),
    });
    es.onmessage = (event: MessageEvent<string>) => {
      if (!event.data) return;
      onEvent(JSON.parse(event.data) as ChangeEvent);
    };
    return () => es.close();
  }
}

export class VelarixClient {
  private readonly baseUrl: string;
  private readonly apiKey: string | null;

  constructor(baseUrl = 'http://localhost:8080/v1', apiKey: string | null = null) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.apiKey = apiKey;
  }

  getBaseUrl(): string {
    return this.baseUrl;
  }

  getApiKey(): string | null {
    return this.apiKey;
  }

  buildHeaders(authOverride?: string | null, includeContentType = true): Record<string, string> {
    const headers: Record<string, string> = {};
    if (includeContentType) headers['Content-Type'] = 'application/json';
    const authValue = authOverride ?? this.apiKey;
    if (authValue) headers.Authorization = `Bearer ${authValue}`;
    return headers;
  }

  session(sessionId: string): VelarixSession {
    return new VelarixSession(this, sessionId);
  }

  async getSessions(): Promise<SessionInfo[]> {
    const res = await fetch(`${this.baseUrl}/sessions`, {
      headers: this.buildHeaders(undefined, false),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async register(email: string, password: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/auth/register`, {
      method: 'POST',
      headers: this.buildHeaders(null),
      body: JSON.stringify({ email, password }),
    });
    if (!res.ok) throw new Error(await res.text());
  }

  async login(email: string, password: string): Promise<string> {
    const res = await fetch(`${this.baseUrl}/auth/login`, {
      method: 'POST',
      headers: this.buildHeaders(null),
      body: JSON.stringify({ email, password }),
    });
    if (!res.ok) throw new Error(await res.text());
    const data = (await res.json()) as { token: string };
    return data.token;
  }

  async listKeys(email: string, authToken?: string | null): Promise<APIKeyRecord[]> {
    const res = await fetch(`${this.baseUrl}/keys`, {
      headers: this.buildHeaders(authToken, false),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async generateKey(email: string, label: string, authToken?: string | null): Promise<APIKeyRecord> {
    const res = await fetch(`${this.baseUrl}/keys/generate`, {
      method: 'POST',
      headers: this.buildHeaders(authToken),
      body: JSON.stringify({ email, label }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async revokeKey(email: string, key: string, authToken?: string | null): Promise<void> {
    const res = await fetch(`${this.baseUrl}/keys/${encodeURIComponent(key)}`, {
      method: 'DELETE',
      headers: this.buildHeaders(authToken, false),
    });
    if (!res.ok) throw new Error(await res.text());
  }
}
