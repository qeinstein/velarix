import os
import asyncio
from typing import Any, Dict, List, Optional
import httpx
from mcp.server.fastmcp import FastMCP

# Initialize FastMCP Server
mcp = FastMCP("Velarix")

VELARIX_BASE_URL = os.getenv("VELARIX_BASE_URL", "http://localhost:8080")
VELARIX_API_KEY = os.getenv("VELARIX_API_KEY", "")

def _headers() -> Dict[str, str]:
    if VELARIX_API_KEY:
        return {"Authorization": f"Bearer {VELARIX_API_KEY}", "Content-Type": "application/json"}
    return {"Content-Type": "application/json"}

@mcp.tool()
async def assert_fact(session_id: str, fact_id: str, payload: dict, justifications: Optional[list] = None, confidence: float = 1.0) -> str:
    """
    Assert a new fact into a Velarix session.
    Provide justifications as a list of lists of Fact IDs if it's derived.
    """
    url = f"{VELARIX_BASE_URL}/v1/s/{session_id}/facts"
    data = {
        "id": fact_id,
        "payload": payload,
        "is_root": not bool(justifications),
        "manual_status": confidence if not justifications else 0.0,
        "justification_sets": justifications or []
    }
    async with httpx.AsyncClient() as client:
        resp = await client.post(url, json=data, headers=_headers())
        resp.raise_for_status()
        return f"Fact {fact_id} successfully asserted."

@mcp.tool()
async def get_fact(session_id: str, fact_id: str) -> str:
    """Retrieve a specific fact from a session."""
    url = f"{VELARIX_BASE_URL}/v1/s/{session_id}/facts/{fact_id}"
    async with httpx.AsyncClient() as client:
        resp = await client.get(url, headers=_headers())
        resp.raise_for_status()
        return resp.text

@mcp.tool()
async def explain_reasoning(session_id: str, fact_id: str, counterfactual_fact_id: Optional[str] = None) -> str:
    """
    Explain the reasoning behind a specific fact. 
    Optionally provide a counterfactual_fact_id to see what would happen if it were removed.
    """
    url = f"{VELARIX_BASE_URL}/v1/s/{session_id}/explain"
    params = {"fact_id": fact_id}
    if counterfactual_fact_id:
        params["counterfactual_fact_id"] = counterfactual_fact_id
        
    async with httpx.AsyncClient() as client:
        resp = await client.get(url, params=params, headers=_headers())
        resp.raise_for_status()
        return resp.text

@mcp.resource("velarix://session/{session_id}/context")
async def get_session_context(session_id: str) -> str:
    """Get the current valid facts for a session to be used as context."""
    url = f"{VELARIX_BASE_URL}/v1/s/{session_id}/slice?format=markdown"
    async with httpx.AsyncClient() as client:
        resp = await client.get(url, headers=_headers())
        resp.raise_for_status()
        return resp.text

if __name__ == "__main__":
    mcp.run()
