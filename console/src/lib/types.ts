export type JSONPrimitive = string | number | boolean | null;
export type JSONValue = JSONPrimitive | JSONValue[] | { [key: string]: JSONValue };

export interface SessionInfo {
  id: string;
  fact_count?: number;
  enforcement_mode?: string;
  status?: 'healthy' | 'warn' | 'strict';
}

export interface User {
  name: string;
  email: string;
  apiKey: string;
  defaultMode: 'strict' | 'warn';
}

export interface UsageStats {
  totalFacts: number;
  activeSessions: number;
  journalEvents: number;
  violationCount: number;
}

export interface APIKeyRecord {
  key: string;
  label: string;
  created_at: number;
  last_used_at: number;
  is_revoked: boolean;
}

export interface Fact {
  id: string;
  payload?: Record<string, JSONValue>;
  metadata?: Record<string, JSONValue>;
  is_root: boolean;
  manual_status: number;
  derived_status: number;
  valid_justification_count: number;
  justification_sets?: string[][];
  resolved_status?: number;
  validation_errors?: string[];
}

export interface ExplanationNode {
  Factid: string;
  Children?: ExplanationNode[] | null;
}

export interface ImpactReport {
  impacted_ids: string[];
  direct_count: number;
  total_count: number;
  action_count: number;
  epistemic_loss: number;
}

export interface ChangeEvent {
  fact_id: string;
  status: number;
  timestamp: number;
}

export interface JournalEntry {
  type: 'assert' | 'invalidate' | 'cycle_violation' | 'snapshot_corruption' | 'confidence_adjusted' | 'revalidation_complete';
  fact?: Fact;
  fact_id?: string;
  payload?: JSONValue;
  timestamp: number;
}
