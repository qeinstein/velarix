import json
from typing import Any, Dict, Optional, Union
from langgraph.checkpoint.base import BaseCheckpointSaver
from langgraph.checkpoint.models import Checkpoint, CheckpointMetadata
from velarix.client import VelarixClient

class VelarixLangGraphMemory(BaseCheckpointSaver):
    """
    A drop-in LangGraph checkpointer that persists state into Velarix.
    Maps thread_id directly to Velarix session_id.
    """
    def __init__(
        self, 
        client: Optional[VelarixClient] = None,
        base_url: str = "http://localhost:8080",
        api_key: Optional[str] = None
    ):
        super().__init__()
        self.client = client or VelarixClient(base_url=base_url, api_key=api_key)

    def get_tuple(self, config: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """Retrieve a checkpoint from Velarix."""
        thread_id = config.get("configurable", {}).get("thread_id")
        if not thread_id:
            return None
        
        session = self.client.session(thread_id)
        try:
            # We store the checkpoint blob in a special system fact
            fact = session.get_fact("_lg_checkpoint")
            checkpoint_data = fact.get("payload", {}).get("checkpoint")
            if not checkpoint_data:
                return None
            
            return {
                "config": config,
                "checkpoint": json.loads(checkpoint_data),
                "metadata": fact.get("payload", {}).get("metadata", {}),
            }
        except:
            return None

    def put(self, config: Dict[str, Any], checkpoint: Checkpoint, metadata: CheckpointMetadata) -> str:
        """Store a checkpoint into Velarix."""
        thread_id = config.get("configurable", {}).get("thread_id")
        if not thread_id:
            return ""
        
        session = self.client.session(thread_id)
        
        # Persist as a system-protected fact
        payload = {
            "checkpoint": json.dumps(checkpoint),
            "metadata": metadata,
            "_system": True # Internal flag for UI filtering
        }
        
        # Note: We use observe here because graph checkpoints are treated as 
        # ground-truth roots for the agent's execution state.
        session.observe("_lg_checkpoint", payload)
        
        return checkpoint.get("ts", "")
