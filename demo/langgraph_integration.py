import os
from typing import TypedDict, List
from langgraph.graph import StateGraph, END
from velarix.integrations.langgraph import VelarixLangGraphMemory
from langchain_openai import ChatOpenAI

# 1. Define State
class AgentState(TypedDict):
    research_topics: List[str]
    findings: List[str]

# 2. Define Nodes
def research_node(state: AgentState):
    topic = state["research_topics"].pop()
    finding = f"Discovered facts about {topic}"
    state["findings"].append(finding)
    return state

# 3. Build Graph
workflow = StateGraph(AgentState)
workflow.add_node("research", research_node)
workflow.set_entry_point("research")
workflow.add_edge("research", END)

# 4. PLUG IN VELARIX (The Drop-in Step)
memory = VelarixLangGraphMemory(
    base_url=os.getenv("VELARIX_BASE_URL", "http://localhost:8080"),
    api_key="vx_your_key_here"
)

app = workflow.compile(checkpointer=memory)

# 5. Execute with Thread ID (maps to Velarix session)
config = {"configurable": {"thread_id": "research_session_001"}}
input_state = {"research_topics": ["Quantum Computing", "Epistemic Logic"], "findings": []}

print("--- Starting Agent Turn 1 ---")
app.invoke(input_state, config)

print("--- Agent Resuming from Velarix Checkpoint ---")
# The next run will pull state directly from Velarix
final_state = app.get_state(config)
print(f"Current Findings in Memory: {final_state.values['findings']}")
