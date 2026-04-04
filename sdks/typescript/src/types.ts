export interface Fact {
  id: string;
  payload?: Record<string, any>;
  is_root: boolean;
  manual_status: number;
  derived_status: number;
  valid_justification_count: number;
  justification_sets?: string[][];
  resolved_status?: number;
}

export interface ExplanationNode {
  Factid: string;
  Children: ExplanationNode[];
}

export interface ChangeEvent {
  fact_id: string;
  status: number;
  timestamp: number;
}

export interface JournalEntry {
  type: 'assert' | 'invalidate' | 'admin_action' | 'decision_record' | 'confidence_adjusted';
  session_id?: string;
  actor_id?: string;
  fact?: Fact;
  fact_id?: string;
  payload?: Record<string, any>;
  timestamp: number;
}

export interface DecisionRecordPayload {
  kind: string;
  [k: string]: any;
}

export interface Decision {
  decision_id: string;
  session_id: string;
  org_id: string;
  fact_id?: string;
  decision_type: string;
  subject_ref: string;
  target_ref: string;
  status: string;
  execution_status: string;
  recommended_action?: string;
  policy_version?: string;
  explanation_summary?: string;
  created_by: string;
  created_at: number;
  updated_at: number;
  last_checked_at?: number;
  executed_at?: number;
  executed_by?: string;
  metadata?: Record<string, any>;
}

export interface DecisionDependency {
  decision_id: string;
  session_id: string;
  fact_id: string;
  dependency_type: string;
  required_status: string;
  current_status?: number;
  source_ref?: string;
  policy_version?: string;
  explanation_hint?: string;
}

export interface DecisionBlocker {
  fact_id: string;
  dependency_type: string;
  required_status: string;
  current_status: number;
  reason_code: string;
  source_ref?: string;
  policy_version?: string;
  explanation_hint?: string;
}

export interface DecisionCheck {
  decision_id: string;
  session_id: string;
  executable: boolean;
  blocked_by: DecisionBlocker[];
  reason_codes: string[];
  checked_at: number;
  decision_version?: number;
  session_version?: number;
  expires_at?: number;
  execution_token?: string;
  explanation_summary?: string;
  dependency_snapshots?: DecisionDependency[];
}

export interface ExplainOptions {
  factId?: string;
  timestamp?: string;
  counterfactualFactId?: string;
}
