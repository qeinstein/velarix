from setuptools import setup, find_packages

setup(
    name="velarix",
    version="0.1.0",
    description="Python SDK for the Velarix belief graph and reasoning engine",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    author="Velarix",
    author_email="team@velarix.dev",
    license="Apache-2.0",
    url="https://github.com/velarix/velarix",
    project_urls={
        "Documentation": "https://velarix.dev/docs",
        "Source": "https://github.com/velarix/velarix",
        "Bug Tracker": "https://github.com/velarix/velarix/issues",
    },
    packages=find_packages(),
    python_requires=">=3.9",
    install_requires=[
        "requests>=2.25.1",
        "httpx>=0.24.0",
        "openai>=1.0.0",
    ],
    extras_require={
        "langchain": ["langchain-core>=1.0.0", "langchain-openai>=1.0.0"],
        "langgraph": ["langgraph>=0.3.0", "langgraph-checkpoint>=2.1.0", "langchain-core>=1.0.0", "langchain-openai>=1.0.0"],
        "llamaindex": ["llama-index>=0.10.0"],
        "crewai": ["crewai>=0.108.0"],
        "local": [],  # Marker for local binary support
    },
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: Apache Software License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Software Development :: Libraries :: Python Modules",
    ],
)
