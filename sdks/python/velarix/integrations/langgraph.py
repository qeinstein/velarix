import asyncio
import base64
import time
from typing import Any, AsyncIterator, Dict, Iterator, List, Optional, Sequence, Tuple

from velarix.client import VelarixClient

try:
    from langgraph.checkpoint.base import (
        BaseCheckpointSaver,
        CheckpointTuple,
        get_checkpoint_id,
        get_checkpoint_metadata,
    )
except ImportError as exc:  # pragma: no cover - optional dependency
    _LANGGRAPH_IMPORT_ERROR = exc

    class VelarixLangGraphMemory:  # type: ignore[no-redef]
        def __init__(self, *_args: Any, **_kwargs: Any) -> None:
            raise ImportError(
                "LangGraph support requires optional dependencies. Install with `pip install velarix[langgraph]`."
            ) from _LANGGRAPH_IMPORT_ERROR

else:
    class VelarixLangGraphMemory(BaseCheckpointSaver[str]):
        EVENT_CHECKPOINT = "langgraph_checkpoint"
        EVENT_WRITES = "langgraph_pending_writes"

        def __init__(
            self,
            *,
            client: Optional[VelarixClient] = None,
            base_url: str = "http://localhost:8080",
            api_key: Optional[str] = None,
            session_id: Optional[str] = None,
            serde: Any = None,
        ) -> None:
            super().__init__(serde=serde)
            self.client = client or VelarixClient(base_url=base_url, api_key=api_key)
            self.default_thread_id = session_id

        def get_tuple(self, config: Dict[str, Any]) -> Optional[CheckpointTuple]:
            thread_id = self._thread_id(config)
            if not thread_id:
                return None
            checkpoint_ns = self._checkpoint_ns(config)
            checkpoints, writes = self._load_checkpoint_records(thread_id)
            checkpoint_id = get_checkpoint_id(config)
            if checkpoint_id:
                record = checkpoints.get((checkpoint_ns, checkpoint_id))
                if record is None:
                    return None
                return self._to_checkpoint_tuple(thread_id, checkpoint_ns, record, writes.get((checkpoint_ns, checkpoint_id), []))

            namespace_records = [record for (ns, _), record in checkpoints.items() if ns == checkpoint_ns]
            if not namespace_records:
                return None
            namespace_records.sort(key=lambda record: (record["created_at_ms"], record["checkpoint_id"]), reverse=True)
            record = namespace_records[0]
            return self._to_checkpoint_tuple(
                thread_id,
                checkpoint_ns,
                record,
                writes.get((checkpoint_ns, record["checkpoint_id"]), []),
            )

        def list(
            self,
            config: Optional[Dict[str, Any]],
            *,
            filter: Optional[Dict[str, Any]] = None,
            before: Optional[Dict[str, Any]] = None,
            limit: Optional[int] = None,
        ) -> Iterator[CheckpointTuple]:
            thread_id = self._thread_id(config)
            if not thread_id:
                return iter(())

            checkpoint_ns = self._checkpoint_ns(config)
            checkpoints, writes = self._load_checkpoint_records(thread_id)
            before_timestamp = self._before_timestamp(checkpoints, checkpoint_ns, before)
            checkpoint_id_filter = get_checkpoint_id(config) if config else None

            items = []
            for (ns, checkpoint_id), record in checkpoints.items():
                if checkpoint_ns and ns != checkpoint_ns:
                    continue
                if checkpoint_id_filter and checkpoint_id != checkpoint_id_filter:
                    continue
                if before_timestamp is not None and record["created_at_ms"] >= before_timestamp:
                    continue
                if filter and not all(record["metadata"].get(key) == value for key, value in filter.items()):
                    continue
                items.append((record["created_at_ms"], checkpoint_id, ns, record))

            items.sort(reverse=True)
            if limit is not None:
                items = items[: max(0, limit)]

            return iter(
                self._to_checkpoint_tuple(thread_id, ns, record, writes.get((ns, checkpoint_id), []))
                for _, checkpoint_id, ns, record in items
            )

        def put(
            self,
            config: Dict[str, Any],
            checkpoint: Dict[str, Any],
            metadata: Dict[str, Any],
            new_versions: Dict[str, Any],
        ) -> Dict[str, Any]:
            thread_id = self._require_thread_id(config)
            checkpoint_ns = self._checkpoint_ns(config)
            payload = {
                "checkpoint_id": checkpoint["id"],
                "checkpoint_ns": checkpoint_ns,
                "checkpoint": self._encode_typed(self.serde.dumps_typed(checkpoint)),
                "metadata": get_checkpoint_metadata(config, metadata),
                "parent_checkpoint_id": config.get("configurable", {}).get("checkpoint_id"),
                "new_versions": new_versions,
                "created_at_ms": int(time.time() * 1000),
            }
            self.client.session(thread_id).append_history(self.EVENT_CHECKPOINT, payload)
            return {
                "configurable": {
                    "thread_id": thread_id,
                    "checkpoint_ns": checkpoint_ns,
                    "checkpoint_id": checkpoint["id"],
                }
            }

        def put_writes(
            self,
            config: Dict[str, Any],
            writes: Sequence[Tuple[str, Any]],
            task_id: str,
            task_path: str = "",
        ) -> None:
            thread_id = self._require_thread_id(config)
            checkpoint_ns = self._checkpoint_ns(config)
            checkpoint_id = self._require_checkpoint_id(config)
            payload = {
                "checkpoint_id": checkpoint_id,
                "checkpoint_ns": checkpoint_ns,
                "task_id": task_id,
                "task_path": task_path,
                "created_at_ms": int(time.time() * 1000),
                "writes": [
                    {
                        "index": idx,
                        "channel": channel,
                        "value": self._encode_typed(self.serde.dumps_typed(value)),
                    }
                    for idx, (channel, value) in enumerate(writes)
                ],
            }
            self.client.session(thread_id).append_history(self.EVENT_WRITES, payload)

        def delete_thread(self, thread_id: str) -> None:
            raise NotImplementedError("VelarixLangGraphMemory does not support destructive thread deletion.")

        def delete_for_runs(self, run_ids: Sequence[str]) -> None:
            raise NotImplementedError("VelarixLangGraphMemory does not support destructive run deletion.")

        def copy_thread(self, source_thread_id: str, target_thread_id: str) -> None:
            source = self.client.session(source_thread_id).get_history()
            target = self.client.session(target_thread_id)
            for entry in source:
                if entry.get("type") not in {self.EVENT_CHECKPOINT, self.EVENT_WRITES}:
                    continue
                target.append_history(entry["type"], entry.get("payload") or {})

        def prune(self, thread_ids: Sequence[str], *, strategy: str = "keep_latest") -> None:
            raise NotImplementedError("VelarixLangGraphMemory does not support checkpoint pruning yet.")

        async def aget_tuple(self, config: Dict[str, Any]) -> Optional[CheckpointTuple]:
            return await asyncio.to_thread(self.get_tuple, config)

        async def alist(
            self,
            config: Optional[Dict[str, Any]],
            *,
            filter: Optional[Dict[str, Any]] = None,
            before: Optional[Dict[str, Any]] = None,
            limit: Optional[int] = None,
        ) -> AsyncIterator[CheckpointTuple]:
            for item in await asyncio.to_thread(lambda: list(self.list(config, filter=filter, before=before, limit=limit))):
                yield item

        async def aput(
            self,
            config: Dict[str, Any],
            checkpoint: Dict[str, Any],
            metadata: Dict[str, Any],
            new_versions: Dict[str, Any],
        ) -> Dict[str, Any]:
            return await asyncio.to_thread(self.put, config, checkpoint, metadata, new_versions)

        async def aput_writes(
            self,
            config: Dict[str, Any],
            writes: Sequence[Tuple[str, Any]],
            task_id: str,
            task_path: str = "",
        ) -> None:
            await asyncio.to_thread(self.put_writes, config, writes, task_id, task_path)

        async def adelete_thread(self, thread_id: str) -> None:
            await asyncio.to_thread(self.delete_thread, thread_id)

        async def adelete_for_runs(self, run_ids: Sequence[str]) -> None:
            await asyncio.to_thread(self.delete_for_runs, run_ids)

        async def acopy_thread(self, source_thread_id: str, target_thread_id: str) -> None:
            await asyncio.to_thread(self.copy_thread, source_thread_id, target_thread_id)

        async def aprune(self, thread_ids: Sequence[str], *, strategy: str = "keep_latest") -> None:
            await asyncio.to_thread(self.prune, thread_ids, strategy=strategy)

        def _thread_id(self, config: Optional[Dict[str, Any]]) -> Optional[str]:
            configurable = (config or {}).get("configurable", {})
            return configurable.get("thread_id") or self.default_thread_id

        def _require_thread_id(self, config: Dict[str, Any]) -> str:
            thread_id = self._thread_id(config)
            if not thread_id:
                raise ValueError("LangGraph checkpoint config must include configurable.thread_id")
            return str(thread_id)

        def _checkpoint_ns(self, config: Optional[Dict[str, Any]]) -> str:
            return str((config or {}).get("configurable", {}).get("checkpoint_ns", "") or "")

        def _require_checkpoint_id(self, config: Dict[str, Any]) -> str:
            checkpoint_id = get_checkpoint_id(config)
            if not checkpoint_id:
                raise ValueError("LangGraph checkpoint writes require configurable.checkpoint_id")
            return str(checkpoint_id)

        def _load_checkpoint_records(
            self,
            thread_id: str,
        ) -> Tuple[Dict[Tuple[str, str], Dict[str, Any]], Dict[Tuple[str, str], List[Dict[str, Any]]]]:
            history = self.client.session(thread_id).get_history()
            checkpoints: Dict[Tuple[str, str], Dict[str, Any]] = {}
            writes: Dict[Tuple[str, str], List[Dict[str, Any]]] = {}

            for entry in history:
                entry_type = entry.get("type")
                payload = entry.get("payload") or {}
                if not isinstance(payload, dict):
                    continue

                checkpoint_id = str(payload.get("checkpoint_id") or "")
                checkpoint_ns = str(payload.get("checkpoint_ns") or "")
                if not checkpoint_id:
                    continue

                key = (checkpoint_ns, checkpoint_id)
                if entry_type == self.EVENT_CHECKPOINT:
                    checkpoints[key] = {
                        "checkpoint_id": checkpoint_id,
                        "checkpoint_ns": checkpoint_ns,
                        "checkpoint": self._decode_typed(payload.get("checkpoint")),
                        "metadata": dict(payload.get("metadata") or {}),
                        "parent_checkpoint_id": payload.get("parent_checkpoint_id") or None,
                        "created_at_ms": int(payload.get("created_at_ms") or entry.get("timestamp") or 0),
                    }
                elif entry_type == self.EVENT_WRITES:
                    writes.setdefault(key, []).append(
                        {
                            "task_id": str(payload.get("task_id") or ""),
                            "task_path": str(payload.get("task_path") or ""),
                            "created_at_ms": int(payload.get("created_at_ms") or entry.get("timestamp") or 0),
                            "writes": list(payload.get("writes") or []),
                        }
                    )

            return checkpoints, writes

        def _before_timestamp(
            self,
            checkpoints: Dict[Tuple[str, str], Dict[str, Any]],
            checkpoint_ns: str,
            before: Optional[Dict[str, Any]],
        ) -> Optional[int]:
            if not before:
                return None
            before_id = get_checkpoint_id(before)
            if not before_id:
                return None
            record = checkpoints.get((checkpoint_ns, before_id))
            if record is None:
                return None
            return int(record["created_at_ms"])

        def _to_checkpoint_tuple(
            self,
            thread_id: str,
            checkpoint_ns: str,
            record: Dict[str, Any],
            write_records: List[Dict[str, Any]],
        ) -> CheckpointTuple:
            pending_writes: List[Tuple[str, str, Any]] = []
            for write_record in sorted(write_records, key=lambda item: item["created_at_ms"]):
                for write in sorted(write_record.get("writes", []), key=lambda item: item.get("index", 0)):
                    pending_writes.append(
                        (
                            write_record["task_id"],
                            str(write.get("channel") or ""),
                            self._decode_typed(write.get("value")),
                        )
                    )

            parent_checkpoint_id = record.get("parent_checkpoint_id")
            return CheckpointTuple(
                config={
                    "configurable": {
                        "thread_id": thread_id,
                        "checkpoint_ns": checkpoint_ns,
                        "checkpoint_id": record["checkpoint_id"],
                    }
                },
                checkpoint=record["checkpoint"],
                metadata=record["metadata"],
                parent_config=(
                    {
                        "configurable": {
                            "thread_id": thread_id,
                            "checkpoint_ns": checkpoint_ns,
                            "checkpoint_id": parent_checkpoint_id,
                        }
                    }
                    if parent_checkpoint_id
                    else None
                ),
                pending_writes=pending_writes,
            )

        def _encode_typed(self, typed_value: Tuple[str, bytes]) -> Dict[str, str]:
            kind, data = typed_value
            return {
                "type": kind,
                "value": base64.b64encode(data).decode("ascii"),
            }

        def _decode_typed(self, encoded: Any) -> Any:
            if not isinstance(encoded, dict):
                raise ValueError("invalid typed payload")
            kind = str(encoded.get("type") or "")
            data = base64.b64decode(str(encoded.get("value") or ""))
            return self.serde.loads_typed((kind, data))


def epistemic_check_node(state: Dict[str, Any], *, client: Optional[VelarixClient] = None, base_url: str = "http://localhost:8080", api_key: Optional[str] = None) -> Dict[str, Any]:
    """Mark the graph state for replanning when the current plan fact is no longer resolved."""
    session_id = state.get("velarix_session_id")
    fact_id = state.get("current_plan_fact_id")
    if not session_id or not fact_id:
        return state

    resolved_client = client or VelarixClient(base_url=base_url, api_key=api_key)
    fact = resolved_client.session(str(session_id)).get_fact(str(fact_id))
    if float(fact.get("resolved_status", 0.0)) < 1.0:
        state["needs_replanning"] = True
    return state
