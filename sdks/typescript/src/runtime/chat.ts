export const RECORD_OBSERVATION_TOOL = 'record_observation';
export const EXPLAIN_REASONING_TOOL = 'explain_reasoning';

const DEFAULT_SYSTEM_PROMPT = 'You are a helpful assistant.';

export function buildSystemInstruction(contextMarkdown: string): string {
  return `

## VELARIX EPISTEMIC PROTOCOL
You are equipped with a memory layer (Velarix). Below are the current justified beliefs in this session.
Use the '${RECORD_OBSERVATION_TOOL}' tool whenever you derive, infer, or assert a new fact that should be remembered.
If your observation depends on any current beliefs, use their exact IDs (e.g., 'fact_123') in the 'justifications' field. Use an OR-of-ANDs format: [[id1, id2], [id3]].

When the user asks you to explain a decision, justify a recommendation, or describe your reasoning, ALWAYS use the '${EXPLAIN_REASONING_TOOL}' tool. Never invent an explanation; narrate exactly what the tool returns, respecting confidence tiers and provenance.

## CURRENT BELIEFS (Velarix)
${contextMarkdown}
---
`;
}

export function velarixTools(): any[] {
  return [
    {
      type: 'function',
      function: {
        name: RECORD_OBSERVATION_TOOL,
        description: 'Persist a new justified belief, observation, or derived plan into long-term memory.',
        parameters: {
          type: 'object',
          properties: {
            id: { type: 'string', description: 'A unique, slugified identifier.' },
            payload: { type: 'object', description: 'The JSON data associated with this fact.' },
            justifications: {
              type: 'array',
              items: { type: 'array', items: { type: 'string' } },
              description: 'List of lists of Fact IDs that justify this observation.'
            },
            confidence: { type: 'number', description: 'Your confidence (0.0 to 1.0).' }
          },
          required: ['id', 'payload']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: EXPLAIN_REASONING_TOOL,
        description: 'Call this tool when the user asks you to explain a decision, justify a recommendation, or describe your reasoning.',
        parameters: {
          type: 'object',
          properties: {
            fact_id: { type: 'string', description: 'The specific belief/fact ID to explain.' },
            timestamp: { type: 'string', description: 'ISO8601 timestamp for point-in-time explanations.' },
            counterfactual_fact_id: {
              type: 'string',
              description: 'A fact ID to hypothetically remove when generating the explanation.'
            }
          }
        }
      }
    }
  ];
}

export function mergeTools(existingTools: any[] = []): any[] {
  const merged = [...existingTools];
  const names = new Set(
    merged
      .filter((tool) => tool && typeof tool === 'object')
      .map((tool) => tool.function?.name)
      .filter(Boolean)
  );
  for (const tool of velarixTools()) {
    if (!names.has(tool.function.name)) {
      merged.push(tool);
    }
  }
  return merged;
}

export function injectSystemInstruction(messages: any[] = [], contextMarkdown: string): any[] {
  const prepared = messages.map((message) => ({ ...message }));
  const instruction = buildSystemInstruction(contextMarkdown);
  const systemIdx = prepared.findIndex((message) => message.role === 'system');
  if (systemIdx !== -1) {
    prepared[systemIdx].content = `${prepared[systemIdx].content ?? ''}${instruction}`;
    return prepared;
  }
  prepared.unshift({ role: 'system', content: DEFAULT_SYSTEM_PROMPT + instruction });
  return prepared;
}

export class VelarixChatRuntime {
  private session: any;
  private source: string;
  private strict: boolean;

  constructor(session: any, options: { source: string; strict?: boolean }) {
    this.session = session;
    this.source = options.source;
    this.strict = options.strict ?? true;
  }

  async prepareParams(params: any): Promise<any> {
    const prepared = { ...(params || {}) };
    const contextMarkdown = await this.session.getSlice('markdown');
    prepared.messages = injectSystemInstruction(prepared.messages || [], contextMarkdown as string);
    prepared.tools = mergeTools(prepared.tools || []);
    if (!('tool_choice' in prepared)) {
      prepared.tool_choice = 'auto';
    }
    return prepared;
  }

  async processResponse(response: any): Promise<any> {
    const errors: string[] = [];
    for (const toolCall of iterToolCalls(response)) {
      try {
        const name = toolCall.function?.name;
        if (name === RECORD_OBSERVATION_TOOL) {
          await this.persistObservation(toolCall, response);
        } else if (name === EXPLAIN_REASONING_TOOL) {
          await this.resolveExplanation(toolCall);
        }
      } catch (error: any) {
        errors.push(`${toolCall.function?.name || 'tool'}: ${error?.message || String(error)}`);
      }
    }
    if (errors.length > 0 && this.strict) {
      throw new Error(errors.join('; '));
    }
    return response;
  }

  private async persistObservation(toolCall: any, response: any): Promise<void> {
    const args = JSON.parse(toolCall.function.arguments || '{}');
    const factId = args.id;
    if (!factId) {
      throw new Error('record_observation requires an id');
    }

    const payload = args.payload || {};
    const justifications = args.justifications;
    let confidence = Number(args.confidence ?? 1);
    payload._provenance = {
      source: this.source,
      model: response?.model ?? null,
      timestamp: response?.created ?? null,
      tool_call_id: toolCall?.id ?? null
    };

    const isRoot = !justifications || justifications.length === 0;
    if (isRoot && confidence > 0.9) {
      await this.session.appendHistory('confidence_adjusted', { original: confidence, adjusted: 0.75 }, factId);
      confidence = 0.75;
    }

    if (justifications && justifications.length > 0) {
      await this.session.derive(factId, justifications, payload);
      return;
    }
    await this.session.observe(factId, payload, undefined, confidence);
  }

  private async resolveExplanation(toolCall: any): Promise<void> {
    const args = JSON.parse(toolCall.function.arguments || '{}');
    const explanation = await this.session.explain({
      factId: args.fact_id,
      timestamp: args.timestamp,
      counterfactualFactId: args.counterfactual_fact_id
    });
    toolCall.function.arguments = JSON.stringify(explanation);
  }
}

function iterToolCalls(response: any): any[] {
  const choices = response?.choices || [];
  if (choices.length === 0) return [];
  return choices[0]?.message?.tool_calls || [];
}
