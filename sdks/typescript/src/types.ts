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
  type: 'assert' | 'invalidate' | 'admin_action' | 'decision_record';
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
