from setuptools import setup, find_packages

setup(
    name="velarix",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "requests>=2.25.1",
        "sseclient-py>=1.7.2",
        "httpx>=0.24.0",
        "openai>=1.0.0"
    ],
    extras_require={
        "langgraph": ["langgraph>=0.0.10", "langchain-openai>=0.1.0"],
        "llamaindex": ["llama-index>=0.10.0"],
        "local": [] # Marker for local binary support
    }
)
