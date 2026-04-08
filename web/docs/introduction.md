---
title: "Introduction"
description: "Project overview and Why Velarix?"
order: 1
---

# Introduction

Welcome to Velarix. This document provides an overview of the project and explains why Velarix exists.

## Project Overview

Velarix gives AI agents a causal memory layer. When a fact changes, stale reasoning is pruned instead of quietly surviving in context. It is designed to track dependencies across facts, beliefs, and decisions, giving models a verifiable layer of state.

## Why Velarix?

Models are good at continuation, but they struggle with un-learning things when premises change. Velarix acts as the layer that notices when a premise changes and removes downstream fiction, ensuring that reasoning chains remain logically consistent over time.

- **Causal Tracking**: Every decision can be traced back through the beliefs that produced it.
- **Correctable State**: Stale reasoning is pruned automatically.
- **Familiar Integration**: Works alongside the runtime and orchestration layers you already use.