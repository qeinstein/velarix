import { EventSourcePolyfill } from 'event-source-polyfill';
import { Fact, ExplanationNode, ChangeEvent, JournalEntry } from './types';

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

  async observe(factId: string, payload?: Record<string, any>): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Content-Type': 'application/json' },
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

  async derive(factId: string, justifications: string[][], payload?: Record<string, any>): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Content-Type': 'application/json' },
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

  async invalidate(factId: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/invalidate`, {
      method: 'POST',
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
  }

  async getSlice(format: 'json' | 'markdown' = 'json', maxFacts: number = 50): Promise<Fact[] | string> {
    const res = await fetch(`${this.baseUrl}/slice?format=${format}&max_facts=${maxFacts}`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return format === 'markdown' ? res.text() : res.json();
  }

  async getFact(factId: string): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getWhy(factId: string): Promise<ExplanationNode[]> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/why`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getImpact(factId: string): Promise<any> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/impact`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getHistory(): Promise<JournalEntry[]> {
    const res = await fetch(`${this.baseUrl}/history`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async revalidate(): Promise<any> {
    const res = await fetch(`${this.baseUrl}/revalidate`, {
      method: 'POST',
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  listen(onEvent: (event: ChangeEvent) => void): () => void {
    const es = new EventSourcePolyfill(`${this.baseUrl}/events`, {
      headers: this.getHeaders()
    });
    es.onmessage = (event) => {
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
}

export class VelarixClient {
  private baseUrl: string;
  private apiKey: string | null;

  constructor(config: VelarixConfig = {}) {
    this.baseUrl = (config.baseUrl || 'http://localhost:8080').replace(/\/$/, '');
    this.apiKey = config.apiKey || null;
  }

  getBaseUrl(): string {
    return this.baseUrl;
  }

  getHeaders(): Record<string, string> {
    return this.apiKey ? { 'Authorization': `Bearer ${this.apiKey}` } : {};
  }

  session(sessionId: string): VelarixSession {
    return new VelarixSession(this, sessionId);
  }

  async getSessions(): Promise<any[]> {
    const res = await fetch(`${this.baseUrl}/v1/sessions`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getUsage(): Promise<Record<string, number>> {
    const res = await fetch(`${this.baseUrl}/v1/org/usage`, {
      headers: this.getHeaders()
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }
}
