from setuptools import setup, find_packages

setup(
    name="velarix",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "requests>=2.25.1",
        "httpx>=0.24.0",
        "openai>=1.0.0"
    ],
    extras_require={
        "langchain": ["langchain-core>=1.0.0", "langchain-openai>=1.0.0"],
        "langgraph": ["langgraph>=0.3.0", "langgraph-checkpoint>=2.1.0", "langchain-core>=1.0.0", "langchain-openai>=1.0.0"],
        "llamaindex": ["llama-index>=0.10.0"],
        "crewai": ["crewai>=0.108.0"],
        "local": [] # Marker for local binary support
    }
)
