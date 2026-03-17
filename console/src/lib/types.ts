export interface SessionInfo {
  id: string;
  fact_count: number;
  enforcement_mode: string;
  status: 'healthy' | 'warn' | 'strict';
}

export interface User {
  name: string;
  email: string;
  apiKey: string;
  defaultMode: 'strict' | 'warn';
}

export interface UsageStats {
  totalFacts: number;
  totalRequests: number;
  activeSessions: number;
}

export interface Fact {
  ID: string;
  payload?: Record<string, any>;
  IsRoot: boolean;
  ManualStatus: number;
  DerivedStatus: number;
  ValidJustificationCount: number;
  justification_sets?: string[][];
  resolved_status?: number;
  validation_errors?: string[];
}

export interface ExplanationNode {
  FactID: string;
  Children: ExplanationNode[];
}

export interface ChangeEvent {
  fact_id: string;
  status: number;
  timestamp: number;
}

export interface JournalEntry {
  type: 'assert' | 'invalidate';
  fact?: Fact;
  fact_id?: string;
  timestamp: number;
}
