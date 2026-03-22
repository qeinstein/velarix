# Velarix: Blast Radius Analysis Overview

**Blast Radius Analysis** is the quantitative measurement of the downstream consequences of a fact retraction. In Velarix, because every belief is part of a Directed Acyclic Graph (DAG) of justifications, you can identify precisely what "must be forgotten" when a premise is invalidated.

## 1. Mathematical Foundation: Dominator Trees
The analysis is powered by the `GetImpact(factID string)` function in the Go kernel. 

*   **The Problem**: A standard graph descendant check shows what *could* be related, but not what *must* be retracted.
*   **The Solution**: Velarix uses **Dominator Tree Ancestry** for $O(1)$ invalidation checks. If Fact B is dominated by Fact A, it means every path from a root to B must pass through A. If A is removed, B's foundation collapses. 
*   **The Benefit**: This is mathematically deterministic, avoiding the "fuzzy" or "stale" context issues common in traditional AI memory (Vector DBs).

## 2. Key Metrics in an Impact Report
When a user queries the impact of a retraction (`GET /v1/s/{id}/facts/{id}/impact`), Velarix returns a report with these dimensions:

| Metric | Business Value |
| :--- | :--- |
| **`TotalCount`** | Scale of impact. How many beliefs become invalid. |
| **`ActionCount`** | **Critical Liability Metric.** Counts how many "actions" (e.g., "Order Lab," "Discharge Patient") were based on the now-false premise. |
| **`DirectCount`** | First-order reasoning impact. Helps visualize the initial layers of a reasoning chain. |
| **`Epistemic Loss`** | Quantifies the "volume of certainty" being removed from the session. |

## 3. Practical Use Case: Correcting Medical Data
Imagine a nurse corrects a patient's Blood Type (Root Fact A). A Blast Radius report instantly shows:
1.  **Direct Impact**: All laboratory orders for that blood type become invalid.
2.  **Transitive Impact**: Any transfusion recommendations based on those orders also fail.
3.  **Action Alerts**: The system flags that a "Transfusion Order" (Action) is now logically unsupported, allowing human intervention *before* the error becomes a patient safety event.

## 4. Product Validation (Why this is the "Right Product")
Standard "AI Safety" attempts usually involve post-process filtering or broad context clearing. Velarix is the only system that provides a **Deterministic Conscience**. 

By exposing the blast radius, you give healthcare providers a "What-If" clinical simulation tool: *"If we retract this lab result, how much of the AI's reasoning do we lose?"* it transforms AI memory into a auditable, reliable system of record.
