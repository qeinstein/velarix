import { VelarixClient } from '../client.js';
import { VelarixChatRuntime } from '../runtime/chat.js';

/**
 * A wrapper for the OpenAI Chat Completions API that automates
 * context injection and fact extraction into Velarix.
 */
export class VelarixOpenAI {
  private velarix: VelarixClient;
  private openai: any; // Using 'any' to avoid strict dependency on 'openai' package versions
  private strict: boolean;

  constructor(openaiClient: any, velarixClient: VelarixClient, options: { strict?: boolean } = {}) {
    this.openai = openaiClient;
    this.velarix = velarixClient;
    this.strict = options.strict ?? true;
  }

  /**
   * Automates the 'Sandwich Pattern':
   * 1. Fetches current Ground Truth from Velarix.
   * 2. Injects it into the system prompt.
   * 3. Injects 'record_observation' and 'explain_reasoning' tools.
   * 4. Executes the OpenAI call.
   * 5. Automatically persists any tool-called observations into the session.
   */
  async chat(sessionId: string, params: any): Promise<any> {
    const session = this.velarix.session(sessionId);
    const runtime = new VelarixChatRuntime(session, {
      source: 'openai_adapter',
      strict: this.strict
    });
    const openaiParams = await runtime.prepareParams(params);
    const response = await this.openai.chat.completions.create(openaiParams);
    return runtime.processResponse(response);
  }
}
