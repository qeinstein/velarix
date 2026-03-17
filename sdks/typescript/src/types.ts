export interface Fact {
  ID: string;
  payload?: Record<string, any>;
  IsRoot: boolean;
  ManualStatus: number;
  DerivedStatus: number;
  ValidJustificationCount: number;
  justification_sets?: string[][];
  resolved_status?: number;
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
