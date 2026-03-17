import { EventSourcePolyfill } from 'event-source-polyfill';
import { Fact, ExplanationNode, ChangeEvent, JournalEntry } from './types';

export class VelarixClient {
  private baseUrl: string;

  constructor(baseUrl: string = 'http://localhost:8080') {
    this.baseUrl = baseUrl.replace(/\/$/, '');
  }

  async observe(factId: string, payload?: Record<string, any>): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
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
      headers: { 'Content-Type': 'application/json' },
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
    const res = await fetch(`${this.baseUrl}/facts/${factId}/invalidate`, { method: 'POST' });
    if (!res.ok) throw new Error(await res.text());
  }

  async getValidContext(): Promise<Fact[]> {
    const res = await fetch(`${this.baseUrl}/facts?valid=true`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getFact(factId: string): Promise<Fact> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getWhy(factId: string): Promise<ExplanationNode[]> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/why`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getImpact(factId: string): Promise<string[]> {
    const res = await fetch(`${this.baseUrl}/facts/${factId}/impact`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getHistory(): Promise<JournalEntry[]> {
    const res = await fetch(`${this.baseUrl}/history`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  listen(onEvent: (event: ChangeEvent) => void): () => void {
    const es = new EventSourcePolyfill(`${this.baseUrl}/events`);
    es.onmessage = (event) => {
      if (event.data) {
        onEvent(JSON.parse(event.data));
      }
    };
    return () => es.close();
  }
}
