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
  type: 'assert' | 'invalidate';
  fact?: Fact;
  fact_id?: string;
  timestamp: number;
}
