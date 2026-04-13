from typing import Any, Callable, Dict, Optional


class VelarixGateway:
    """Record tool activity to Velarix while tolerating transient write failures."""

    def __init__(self, session, mode: str = "strict", max_buffered: int = 2000):
        self.session = session
        self.mode = mode or "strict"
        self.max_buffered = max(1, int(max_buffered))
        self._buffer = []

    def flush(self) -> None:
        """Replay any buffered decision events through the attached session."""
        if not self._buffer:
            return
        pending = list(self._buffer)
        self._buffer = []
        for kind, payload in pending:
            self.session.record_decision(kind, payload)

    def _record(self, kind: str, payload: Dict[str, Any]) -> None:
        try:
            if self.mode == "buffered":
                self.flush()
            self.session.record_decision(kind, payload)
        except Exception:
            if self.mode != "buffered":
                raise
            if len(self._buffer) >= self.max_buffered:
                raise
            self._buffer.append((kind, payload))

    def call_tool(
        self,
        tool: str,
        input: Dict[str, Any],
        fn: Callable[[Dict[str, Any]], Any],
        trace_id: Optional[str] = None,
        tags: Optional[list] = None,
    ) -> Any:
        """Record a tool call, execute it, and persist either the result or the error."""
        self._record("tool_call", {"trace_id": trace_id, "tool": tool, "input": input, "tags": tags or []})
        try:
            output = fn(input)
            self._record(
                "tool_result",
                {"trace_id": trace_id, "tool": tool, "input": input, "output": output, "tags": tags or []},
            )
            return output
        except Exception as e:
            self._record("error", {"trace_id": trace_id, "tool": tool, "input": input, "error": str(e), "tags": tags or []})
            raise
