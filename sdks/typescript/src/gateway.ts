import type { DecisionRecordPayload } from "./types.js";
import type { VelarixSession } from "./client.js";

export class VelarixGateway {
  private session: VelarixSession;
  private mode: "strict" | "buffered";
  private buffer: DecisionRecordPayload[] = [];
  private maxBuffered: number;

  constructor(session: VelarixSession, opts?: { mode?: "strict" | "buffered"; maxBuffered?: number }) {
    this.session = session;
    this.mode = opts?.mode ?? "strict";
    this.maxBuffered = typeof opts?.maxBuffered === "number" && opts.maxBuffered > 0 ? Math.floor(opts.maxBuffered) : 2000;
  }

  async flush(): Promise<void> {
    if (this.buffer.length === 0) return;
    const pending = [...this.buffer];
    this.buffer = [];
    for (const p of pending) {
      await this.session.recordDecision(p);
    }
  }

  private async record(payload: DecisionRecordPayload): Promise<void> {
    try {
      if (this.mode === "buffered") await this.flush();
      await this.session.recordDecision(payload);
    } catch (e) {
      if (this.mode !== "buffered") throw e;
      if (this.buffer.length >= this.maxBuffered) {
        // Fail closed once buffering limit is hit.
        throw e;
      }
      this.buffer.push(payload);
    }
  }

  async callTool<TInput extends Record<string, any>, TOutput>(
    tool: string,
    input: TInput,
    fn: (input: TInput) => Promise<TOutput>,
    opts?: { traceId?: string; tags?: string[] },
  ): Promise<TOutput> {
    const trace_id = opts?.traceId;
    const tags = opts?.tags;

    await this.record({
      kind: "tool_call",
      trace_id,
      tool,
      input,
      tags,
    } as DecisionRecordPayload);

    try {
      const output = await fn(input);
      await this.record({
        kind: "tool_result",
        trace_id,
        tool,
        input,
        output,
        tags,
      } as DecisionRecordPayload);
      return output;
    } catch (e: any) {
      await this.record({
        kind: "error",
        trace_id,
        tool,
        input,
        error: String(e?.message || e),
        tags,
      } as DecisionRecordPayload);
      throw e;
    }
  }
}
