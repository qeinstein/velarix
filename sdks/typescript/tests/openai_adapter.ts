import { VelarixOpenAI } from "../src/adapters/openai.js";

declare const process: { exit(code?: number): void };

class FakeSession {
  public observed: any[] = [];
  public derived: any[] = [];
  public history: any[] = [];

  async getSlice(_format: "markdown" | "json" = "markdown"): Promise<string> {
    return "## Fact: F1\n```json\n{}\n```";
  }

  async observe(factId: string, payload?: Record<string, any>, _idempotencyKey?: string, confidence: number = 1): Promise<any> {
    this.observed.push({ factId, payload, confidence });
    return {};
  }

  async derive(factId: string, justifications: string[][], payload?: Record<string, any>): Promise<any> {
    this.derived.push({ factId, justifications, payload });
    return {};
  }

  async appendHistory(type: string, payload?: Record<string, any>, factId?: string): Promise<any> {
    this.history.push({ type, payload, factId });
    return {};
  }

  async explain(options: Record<string, any> = {}): Promise<any> {
    return { explanation: "ok", ...options };
  }
}

class FakeVelarixClient {
  private readonly fakeSession: FakeSession;

  constructor(fakeSession: FakeSession) {
    this.fakeSession = fakeSession;
  }

  session(_sessionId: string): FakeSession {
    return this.fakeSession;
  }
}

class FakeOpenAIClient {
  public requests: any[] = [];
  private readonly response: any;
  public chat: {
    completions: {
      create: (params: any) => Promise<any>;
    };
  };

  constructor(response: any) {
    this.response = response;
    this.chat = {
      completions: {
        create: async (_params: any) => this.response
      }
    };
  }
}

function createFakeOpenAIClient(response: any): FakeOpenAIClient {
  const client = new FakeOpenAIClient(response);
  client.chat.completions.create = async (params: any) => {
    client.requests.push(params);
    return response;
  };
  return client;
}

async function testParallelCalls(): Promise<void> {
  const session = new FakeSession();
  const response = {
    model: "gpt-4o",
    created: 123,
    choices: [
      {
        message: {
          tool_calls: [
            {
              id: "call_1",
              function: {
                name: "record_observation",
                arguments: JSON.stringify({ id: "obs_1", payload: { data: "A" } })
              }
            },
            {
              id: "call_2",
              function: {
                name: "record_observation",
                arguments: JSON.stringify({ id: "obs_2", payload: { data: "B" }, justifications: [["obs_1"]] })
              }
            }
          ]
        }
      }
    ]
  };
  const openai = createFakeOpenAIClient(response);
  const adapter = new VelarixOpenAI(openai as any, new FakeVelarixClient(session) as any);

  await adapter.chat("test-session", { model: "gpt-4o", messages: [{ role: "user", content: "Test" }] });

  if (!String(openai.requests[0]?.messages?.[0]?.content || "").includes("VELARIX EPISTEMIC PROTOCOL")) {
    throw new Error("system instruction was not injected");
  }
  if (session.observed[0]?.factId !== "obs_1") {
    throw new Error("root observation was not persisted");
  }
  if (session.observed[0]?.confidence !== 0.75) {
    throw new Error("root confidence was not downgraded");
  }
  if (session.derived[0]?.factId !== "obs_2") {
    throw new Error("derived observation was not persisted");
  }
  console.log("PASS: ts openai adapter parallel calls");
}

async function testExplainResolution(): Promise<void> {
  const session = new FakeSession();
  const response = {
    model: "gpt-4o",
    created: 456,
    choices: [
      {
        message: {
          tool_calls: [
            {
              id: "call_explain",
              function: {
                name: "explain_reasoning",
                arguments: JSON.stringify({ fact_id: "fact_123" })
              }
            }
          ]
        }
      }
    ]
  };
  const openai = createFakeOpenAIClient(response);
  const adapter = new VelarixOpenAI(openai as any, new FakeVelarixClient(session) as any);

  const finalResponse = await adapter.chat("test-session", { model: "gpt-4o", messages: [{ role: "user", content: "Explain" }] });
  const resolved = JSON.parse(finalResponse.choices[0].message.tool_calls[0].function.arguments);
  if (resolved.factId !== "fact_123") {
    throw new Error("explain output was not resolved through session.explain()");
  }
  console.log("PASS: ts openai adapter explain resolution");
}

async function main(): Promise<void> {
  await testParallelCalls();
  await testExplainResolution();
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
