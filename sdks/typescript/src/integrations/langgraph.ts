import { VelarixClient } from '../client.js';

/**
 * Interface for LangGraph Checkpoint
 */
export interface Checkpoint {
  ts: string;
  channel_values: Record<string, any>;
  channel_versions: Record<string, any>;
  versions_seen: Record<string, any>;
  pending_sends: any[];
}

export interface CheckpointMetadata {
  source: string;
  step: number;
  writes: Record<string, any>;
  [key: string]: any;
}

export interface CheckpointTuple {
  config: Record<string, any>;
  checkpoint: Checkpoint;
  metadata: CheckpointMetadata;
}

/**
 * A TypeScript implementation of the LangGraph checkpointer for Velarix.
 * Note: LangGraph JS/TS uses different base classes, so this is designed 
 * to be compatible with the LangChain/LangGraph.js BaseCheckpointSaver interface.
 */
export class VelarixLangGraphMemory {
  private client: VelarixClient;

  constructor(clientOrConfig: VelarixClient | { baseUrl?: string; apiKey?: string }) {
    if (clientOrConfig instanceof VelarixClient) {
      this.client = clientOrConfig;
    } else {
      this.client = new VelarixClient(clientOrConfig);
    }
  }

  /**
   * Retrieve a checkpoint from Velarix.
   */
  async getTuple(config: Record<string, any>): Promise<CheckpointTuple | undefined> {
    const thread_id = config.configurable?.thread_id;
    if (!thread_id) return undefined;

    const session = this.client.session(thread_id);
    try {
      const fact = await session.getFact('_lg_checkpoint');
      const checkpointData = fact.payload?.checkpoint;
      if (!checkpointData) return undefined;

      return {
        config,
        checkpoint: typeof checkpointData === 'string' ? JSON.parse(checkpointData) : checkpointData,
        metadata: fact.payload?.metadata || {},
      };
    } catch (e) {
      return undefined;
    }
  }

  /**
   * Store a checkpoint into Velarix.
   */
  async put(config: Record<string, any>, checkpoint: Checkpoint, metadata: CheckpointMetadata): Promise<string> {
    const thread_id = config.configurable?.thread_id;
    if (!thread_id) return '';

    const session = this.client.session(thread_id);

    const payload = {
      checkpoint: JSON.stringify(checkpoint),
      metadata,
      _system: true
    };

    await session.observe('_lg_checkpoint', payload);
    return checkpoint.ts || '';
  }
}
