import os

from velarix import VelarixClient
from velarix.integrations.crewai import VelarixCrewAIMemory

try:
    from crewai import Agent, Crew, Process, Task
except ImportError as exc:  # pragma: no cover - demo helper
    raise SystemExit("Install CrewAI support with `pip install velarix[crewai] crewai`.") from exc


client = VelarixClient(
    base_url=os.getenv("VELARIX_BASE_URL", "http://localhost:8080"),
    api_key=os.getenv("VELARIX_API_KEY", "vx_your_key_here"),
)
session = client.session("crew_demo_session")

session.observe("invoice_ready", {"summary": "Invoice 42 is ready for release"})
session.observe("vendor_verified", {"summary": "Vendor 7 passed KYB"})

memory = VelarixCrewAIMemory(session, max_facts=40, max_chars=6000)

researcher = Agent(
    role="Finance analyst",
    goal="Summarize whether release blockers remain for the payment approval",
    backstory="You produce compact, auditable release summaries.",
    verbose=True,
)

task = Task(
    description=memory.augment_description(
        "Review the current payment approval state and produce a short release recommendation.",
        query="payment approval blockers invoice vendor verification",
    ),
    expected_output="A short recommendation with blockers or a release decision.",
    agent=researcher,
)

crew = Crew(agents=[researcher], tasks=[task], process=Process.sequential)
result = crew.kickoff()
print(result)

memory.record_observation(
    "crew_release_recommendation",
    {"summary": str(result)},
    justifications=[["invoice_ready", "vendor_verified"]],
)
