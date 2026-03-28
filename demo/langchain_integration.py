import os

from langchain_core.messages import HumanMessage
from langchain_openai import ChatOpenAI

from velarix.client import VelarixClient
from velarix.integrations.langchain import VelarixLangChainChatModel


client = VelarixClient(
    base_url=os.getenv("VELARIX_BASE_URL", "http://localhost:8080"),
    api_key="vx_your_key_here",
)
session = client.session("langchain_session_001")

base_model = ChatOpenAI(model="gpt-4o-mini", api_key=os.getenv("OPENAI_API_KEY"))
model = VelarixLangChainChatModel(model=base_model, session=session)

response = model.invoke([HumanMessage(content="Summarize the latest validated facts in memory.")])
print(response.content)
