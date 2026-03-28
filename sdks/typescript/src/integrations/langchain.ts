import { AIMessage, ChatMessage, HumanMessage, SystemMessage, ToolMessage } from "@langchain/core/messages";
import { VelarixSession } from "../client.js";
import { VelarixChatRuntime } from "../runtime/chat.js";

export interface VelarixLangChainOptions {
  source?: string;
  strict?: boolean;
  boundTools?: any[];
  bindOptions?: Record<string, any>;
}

export class VelarixLangChainChatModel {
  private model: any;
  private session: VelarixSession;
  private source: string;
  private strict: boolean;
  private boundTools: any[];
  private bindOptions: Record<string, any>;

  constructor(model: any, session: VelarixSession, options: VelarixLangChainOptions = {}) {
    this.model = model;
    this.session = session;
    this.source = options.source ?? "langchainjs_adapter";
    this.strict = options.strict ?? true;
    this.boundTools = [...(options.boundTools ?? [])];
    this.bindOptions = { ...(options.bindOptions ?? {}) };
  }

  bindTools(tools: any[], options: Record<string, any> = {}): VelarixLangChainChatModel {
    return new VelarixLangChainChatModel(this.model, this.session, {
      source: this.source,
      strict: this.strict,
      boundTools: [...tools],
      bindOptions: { ...options }
    });
  }

  async invoke(input: any, options: Record<string, any> = {}): Promise<any> {
    const runtime = new VelarixChatRuntime(this.session, {
      source: this.source,
      strict: this.strict
    });
    const prepared = await runtime.prepareParams({
      messages: this.normalizeToOpenAIMessages(input),
      tools: this.boundTools,
      tool_choice: this.bindOptions.tool_choice
    });
    const boundModel = this.bindWrappedModel(prepared.tools, prepared.tool_choice);
    const response = await boundModel.invoke(this.toLangChainMessages(prepared.messages), options);
    await runtime.processResponse(this.toOpenAILikeResponse(response));
    return response;
  }

  async ainvoke(input: any, options: Record<string, any> = {}): Promise<any> {
    return this.invoke(input, options);
  }

  private bindWrappedModel(tools: any[], toolChoice: any): any {
    if (typeof this.model?.bindTools !== "function") {
      throw new TypeError("Wrapped LangChainJS model must support bindTools().");
    }
    const bindOptions = { ...this.bindOptions };
    if (toolChoice !== undefined && bindOptions.tool_choice === undefined) {
      bindOptions.tool_choice = toolChoice;
    }
    return this.model.bindTools(tools ?? [], bindOptions);
  }

  private normalizeToOpenAIMessages(input: any): any[] {
    if (typeof input === "string") {
      return [{ role: "user", content: input }];
    }
    if (Array.isArray(input)) {
      return input.map((message) => this.toOpenAIMessage(message));
    }
    if (input && typeof input.toChatMessages === "function") {
      const messages = input.toChatMessages();
      if (Array.isArray(messages)) {
        return messages.map((message: any) => this.toOpenAIMessage(message));
      }
    }
    throw new TypeError("VelarixLangChainChatModel.invoke expects a string, BaseMessage[], or chat prompt value.");
  }

  private toOpenAIMessage(message: any): Record<string, any> {
    const type = typeof message?.getType === "function" ? message.getType() : undefined;
    const content = message?.content ?? "";

    if (type === "human") {
      return { role: "user", content };
    }
    if (type === "system") {
      return { role: "system", content };
    }
    if (type === "tool") {
      return {
        role: "tool",
        content,
        tool_call_id: message?.tool_call_id ?? message?.toolCallId ?? ""
      };
    }
    if (type === "ai") {
      return {
        role: "assistant",
        content,
        tool_calls: ((message?.tool_calls ?? []) as any[]).map((toolCall) => ({
          id: toolCall.id,
          type: "function",
          function: {
            name: toolCall.name,
            arguments: JSON.stringify(toolCall.args ?? {})
          }
        }))
      };
    }
    if (typeof message?.role === "string") {
      return { role: message.role, content };
    }
    return { role: "user", content };
  }

  private toLangChainMessages(messages: Record<string, any>[]): any[] {
    return messages.map((message) => {
      const role = message.role;
      const content = message.content ?? "";
      if (role === "system") {
        return new SystemMessage(content);
      }
      if (role === "assistant") {
        return new AIMessage({
          content,
          tool_calls: (message.tool_calls ?? []).map((toolCall: any) => this.toLangChainToolCall(toolCall))
        });
      }
      if (role === "tool") {
        return new ToolMessage({
          content,
          tool_call_id: message.tool_call_id ?? ""
        });
      }
      if (role === "user") {
        return new HumanMessage(content);
      }
      return new ChatMessage({ role: role ?? "user", content });
    });
  }

  private toLangChainToolCall(toolCall: any): any {
    const fn = toolCall?.function ?? {};
    let args = fn.arguments ?? {};
    if (typeof args === "string") {
      try {
        args = JSON.parse(args);
      } catch {
        args = { raw_arguments: args };
      }
    }
    return {
      name: fn.name,
      args,
      id: toolCall?.id,
      type: "tool_call"
    };
  }

  private toOpenAILikeResponse(message: any): any {
    const toolCalls = ((message?.tool_calls ?? []) as any[]).map((toolCall) => ({
      id: toolCall.id,
      function: {
        name: toolCall.name,
        arguments: JSON.stringify(toolCall.args ?? {})
      }
    }));
    return {
      model: this.model?.model ?? this.model?.modelName ?? this.model?.constructor?.name ?? "langchainjs_model",
      created: Math.floor(Date.now() / 1000),
      choices: [{ message: { tool_calls: toolCalls } }]
    };
  }
}

export function wrapLangChainModel(
  model: any,
  session: VelarixSession,
  options: VelarixLangChainOptions = {}
): VelarixLangChainChatModel {
  return new VelarixLangChainChatModel(model, session, options);
}
