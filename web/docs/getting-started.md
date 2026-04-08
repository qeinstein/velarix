---
title: "Getting Started"
description: "Installation, prerequisites, and a Quick Start guide."
order: 2
---

# Getting Started

This guide will walk you through the prerequisites, installation process, and a quick start to get you up and running with Velarix.

## Prerequisites

Before you begin, ensure you have the following installed:

- Node.js (v18 or higher)
- npm or yarn
- Go (if running the backend locally)
- Docker (optional, for self-hosted deployment)

## Installation

To integrate Velarix into your existing project, replace your standard OpenAI import with the Velarix adapter.

\`\`\`python
# Standard client
from openai import OpenAI
client = OpenAI()

# With Velarix
from velarix.adapters.openai import OpenAI
client = OpenAI(velarix_session_id="research-1")
\`\`\`

## Quick Start

1. **Start the local server:**

   \`\`\`bash
   go run main.go
   \`\`\`

2. **Initialize the client:**

   \`\`\`python
   from velarix.client import VelarixClient

   client = VelarixClient(
       base_url="http://localhost:8080",
       api_key="your-api-key"
   )
   session = client.session("my-first-session")
   \`\`\`

3. **Make an observation:**

   \`\`\`python
   session.observe("user_authenticated", {"user_id": "123"})
   \`\`\`