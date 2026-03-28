import { VelarixClient } from '../client.js';
import type { Fact } from '../types.js';

/**
 * Basic Node interface matching LlamaIndex.ts structure.
 */
export interface NodeWithScore {
  node: {
    text: string;
    id_: string;
    metadata: Record<string, any>;
  };
  score: number;
}

/**
 * A LlamaIndex.ts compatible retriever for Velarix.
 */
export class VelarixRetriever {
  private client: VelarixClient;
  private sessionId: string;
  private maxFacts: number;

  constructor(
    sessionId: string,
    clientOrConfig: VelarixClient | { baseUrl?: string; apiKey?: string },
    maxFacts: number = 20
  ) {
    this.sessionId = sessionId;
    this.maxFacts = maxFacts;
    if (clientOrConfig instanceof VelarixClient) {
      this.client = clientOrConfig;
    } else {
      this.client = new VelarixClient(clientOrConfig);
    }
  }

  /**
   * Fetch logically 'Valid' facts from Velarix and convert to LlamaIndex-compatible nodes.
   */
  async retrieve(query: string): Promise<NodeWithScore[]> {
    const session = this.client.session(this.sessionId);
    const factsRaw = await session.getSlice('json', this.maxFacts);
    
    // Ensure facts is an array
    const facts = Array.isArray(factsRaw) ? factsRaw : [];

    return facts.map((fact: Fact) => {
      const payloadStr = JSON.stringify(fact.payload || {});
      return {
        node: {
          text: `Fact ID: ${fact.id}\nContent: ${payloadStr}`,
          id_: fact.id,
          metadata: fact.payload || {},
        },
        // All logically 'Valid' facts are considered primary matches
        score: 1.0,
      };
    });
  }
}
