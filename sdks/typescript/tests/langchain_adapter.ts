import { AIMessage, HumanMessage } from "@langchain/core/messages";
import { VelarixLangChainChatModel } from "../src/integrations/langchain.js";

declare const process: { exit(code?: number): void };

class FakeSession {
  public observed: any[] = [];
  public history: any[] = [];

  async getSlice(_format: "markdown" | "json" = "markdown"): Promise<string> {
    return "## Fact: TS1\n```json\n{}\n```";
  }

  async observe(factId: string, payload?: Record<string, any>, _idempotencyKey?: string, confidence: number = 1): Promise<any> {
    this.observed.push({ factId, payload, confidence });
    return {};
  }

  async derive(factId: string, justifications: string[][], payload?: Record<string, any>): Promise<any> {
    this.observed.push({ factId, justifications, payload });
    return {};
  }

  async appendHistory(type: string, payload?: Record<string, any>, factId?: string): Promise<any> {
    this.history.push({ type, payload, factId });
    return {};
  }

  async explain(): Promise<any> {
    return { explanation: "ok" };
  }
}

class FakeToolCapableModel {
  public boundTools: any[] = [];
  public boundOptions: Record<string, any> = {};
  public invocations: any[] = [];
  public modelName = "fake-langchainjs-model";

  bindTools(tools: any[], options: Record<string, any> = {}): FakeToolCapableModel {
    this.boundTools = [...tools];
    this.boundOptions = { ...options };
    return this;
  }

  async invoke(messages: any[], options: Record<string, any> = {}): Promise<AIMessage> {
    this.invocations.push({ messages, options });
    return new AIMessage({
      content: "stored",
      tool_calls: [
        {
          name: "record_observation",
          args: { id: "ts_obs", payload: { topic: "LangChainJS" } },
          id: "call_ts_1",
          type: "tool_call"
        }
      ]
    });
  }
}

async function main(): Promise<void> {
  const session = new FakeSession() as any;
  const baseModel = new FakeToolCapableModel();
  const model = new VelarixLangChainChatModel(baseModel as any, session);

  const response = await model.invoke([new HumanMessage("Test LangChainJS")]);
  if (!(response instanceof AIMessage)) {
    throw new Error("Expected AIMessage response");
  }
  if (!baseModel.boundTools.some((tool) => tool.function?.name === "record_observation")) {
    throw new Error("record_observation tool was not bound");
  }
  if (!baseModel.boundTools.some((tool) => tool.function?.name === "explain_reasoning")) {
    throw new Error("explain_reasoning tool was not bound");
  }
  if (!String(baseModel.invocations[0]?.messages?.[0]?.content || "").includes("VELARIX EPISTEMIC PROTOCOL")) {
    throw new Error("system instruction was not injected");
  }
  if (session.observed[0]?.factId !== "ts_obs") {
    throw new Error("observation was not persisted");
  }
  if (session.observed[0]?.confidence !== 0.75) {
    throw new Error("root confidence was not downgraded");
  }
  console.log("PASS: langchainjs adapter");
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
