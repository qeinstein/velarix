import { EventSourcePolyfill } from 'event-source-polyfill';
import type { Fact, ExplanationNode, ChangeEvent, JournalEntry, SessionInfo } from './types';

export class VelarixSession {
  private baseUrl: string;
  private client: VelarixClient;

  constructor(client: VelarixClient, sessionId: string) {
    this.client = client;
    this.baseUrl = `${client.getBaseUrl()}/s/${sessionId}`;
  }

  private _headers(): Record<string, string> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    const apiKey = this.client.getApiKey();
    if (apiKey) {
      headers['Authorization'] = `Bearer ${apiKey}`;
    }
    return headers;
  }

  async observe(factId: string, payload?: Record<string, any>): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: this._headers(),
      body: JSON.stringify({
        ID: factId,
        IsRoot: true,
        ManualStatus: 1,
        payload: payload || {}
      })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async derive(factId: string, justifications: string[][], payload?: Record<string, any>): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: this._headers(),
      body: JSON.stringify({
        ID: factId,
        IsRoot: false,
        justification_sets: justifications,
        payload: payload || {}
      })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async invalidate(factId: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/invalidate`, { 
      method: 'POST',
      headers: this._headers()
    });
    if (!res.ok) throw new Error(await res.text());
  }

  async setConfig(schema?: string, mode?: 'strict' | 'warn'): Promise<any> {
    const res = await fetch(`${this.baseUrl}/config`, {
      method: 'POST',
      headers: this._headers(),
      body: JSON.stringify({
        schema,
        enforcement_mode: mode
      })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getSlice(format: 'json' | 'markdown' = 'json', maxFacts: number = 50): Promise<any> {
    const res = await fetch(`${this.baseUrl}/slice?format=${format}&max_facts=${maxFacts}`, {
      headers: this._headers()
    });
    if (!res.ok) throw new Error(await res.text());
    if (format === 'markdown') return res.text();
    return res.json();
  }

  async getFacts(validOnly: boolean = false): Promise<Fact[]> {
    const res = await fetch(`${this.baseUrl}/facts${validOnly ? '?valid=true' : ''}`, {
      headers: this._headers()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getFact(factId: string): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}`, {
      headers: this._headers()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getWhy(factId: string): Promise<ExplanationNode[]> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/why`, {
      headers: this._headers()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getImpact(factId: string): Promise<string[]> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/impact`, {
      headers: this._headers()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getHistory(): Promise<JournalEntry[]> {
    const res = await fetch(`${this.baseUrl}/history`, {
      headers: this._headers()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  listen(onEvent: (event: ChangeEvent) => void): () => void {
    const es = new EventSourcePolyfill(`${this.baseUrl}/events`, {
      headers: this._headers()
    });
    es.onmessage = (event: any) => {
      if (event.data) {
        onEvent(JSON.parse(event.data));
      }
    };
    return () => es.close();
  }
}

export class VelarixClient {
  private baseUrl: string;
  private apiKey: string | null;

  constructor(baseUrl: string = 'http://localhost:8080', apiKey: string | null = null) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.apiKey = apiKey;
  }

  getBaseUrl(): string {
    return this.baseUrl;
  }

  getApiKey(): string | null {
    return this.apiKey;
  }

  session(sessionId: string): VelarixSession {
    return new VelarixSession(this, sessionId);
  }

  async getSessions(): Promise<SessionInfo[]> {
    const headers: Record<string, string> = {};
    if (this.apiKey) {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }
    const res = await fetch(`${this.baseUrl}/sessions`, { headers });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }
}
