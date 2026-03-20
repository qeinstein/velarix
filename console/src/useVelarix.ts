import { useCallback, useEffect, useMemo, useState } from 'react';
import { VelarixClient } from './lib/client';
import type {
  ChangeEvent,
  ExplanationNode,
  Fact,
  ImpactReport,
  JournalEntry,
  SessionInfo,
} from './lib/types';

const EMPTY_IMPACT: ImpactReport = {
  impacted_ids: [],
  direct_count: 0,
  total_count: 0,
  action_count: 0,
  epistemic_loss: 0,
};

function indexFacts(data: Fact[]): Record<string, Fact> {
  return data.reduce<Record<string, Fact>>((acc, fact) => {
    acc[fact.ID] = fact;
    return acc;
  }, {});
}

function flattenExplanation(nodes: ExplanationNode[], acc = new Set<string>()): Set<string> {
  for (const node of nodes) {
    acc.add(node.FactID);
    if (node.Children?.length) flattenExplanation(node.Children, acc);
  }
  return acc;
}

export function useVelarix(url = 'http://localhost:8080/v1') {
  const [apiKey, setApiKey] = useState(() => localStorage.getItem('velarix_api_key') || '');
  const [sessionId, setSessionId] = useState(() => localStorage.getItem('velarix_session_id') || 'default-session');
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [facts, setFacts] = useState<Record<string, Fact>>({});
  const [history, setHistory] = useState<JournalEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [authRequired, setAuthRequired] = useState(!apiKey);

  const client = useMemo(() => new VelarixClient(url, apiKey || null), [url, apiKey]);
  const session = useMemo(() => client.session(sessionId), [client, sessionId]);

  const activeSessionInfo = useMemo(
    () => sessions.find((entry) => entry.id === sessionId),
    [sessions, sessionId],
  );

  useEffect(() => {
    localStorage.setItem('velarix_session_id', sessionId);
  }, [sessionId]);

  const refreshSessions = useCallback(async () => {
    if (!apiKey) {
      setSessions([]);
      setAuthRequired(true);
      return;
    }

    try {
      const nextSessions = await client.getSessions();
      setSessions(Array.isArray(nextSessions) ? nextSessions : []);
      setAuthRequired(false);
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load sessions';
      if (message.includes('401') || message.toLowerCase().includes('invalid or expired api key')) {
        setAuthRequired(true);
      } else {
        setError(message);
      }
    }
  }, [apiKey, client]);

  const refreshHistory = useCallback(async () => {
    if (!apiKey) {
      setHistory([]);
      return;
    }

    try {
      const nextHistory = await session.getHistory();
      setHistory(Array.isArray(nextHistory) ? nextHistory : []);
    } catch (err) {
      console.error('Failed to load history:', err);
    }
  }, [apiKey, session]);

  const loadFacts = useCallback(async () => {
    if (!apiKey) {
      setFacts({});
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      const data = await session.getFacts();
      setFacts(Array.isArray(data) ? indexFacts(data) : {});
      setError(null);
      setAuthRequired(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load facts';
      if (message.includes('401') || message.toLowerCase().includes('invalid or expired api key')) {
        setAuthRequired(true);
      } else {
        setError(message);
      }
    } finally {
      setLoading(false);
    }
  }, [apiKey, session]);

  const connect = useCallback((key: string) => {
    localStorage.setItem('velarix_api_key', key);
    setApiKey(key);
    setAuthRequired(false);
    setError(null);
  }, []);

  const disconnect = useCallback(() => {
    localStorage.removeItem('velarix_api_key');
    setApiKey('');
    setSessions([]);
    setFacts({});
    setHistory([]);
    setAuthRequired(true);
    setError(null);
  }, []);

  useEffect(() => {
    if (!apiKey) {
      setLoading(false);
      setAuthRequired(true);
      setSessions([]);
      setFacts({});
      setHistory([]);
      return;
    }

    refreshSessions();
  }, [apiKey, refreshSessions]);

  useEffect(() => {
    if (!apiKey) return;
    loadFacts();
    refreshHistory();
  }, [apiKey, loadFacts, refreshHistory, sessionId]);

  useEffect(() => {
    if (!apiKey) return undefined;

    let unsubscribe: (() => void) | null = null;
    try {
      unsubscribe = session.listen((event: ChangeEvent) => {
        setFacts((prev) => {
          const target = prev[event.fact_id];
          if (!target) return prev;
          return {
            ...prev,
            [event.fact_id]: {
              ...target,
              resolved_status: event.status,
            },
          };
        });
        refreshHistory();
        refreshSessions();
      });
    } catch (err) {
      console.error('SSE connection failed:', err);
    }

    return () => {
      if (unsubscribe) unsubscribe();
    };
  }, [apiKey, refreshHistory, refreshSessions, session]);

  const invalidateFact = useCallback(async (id: string) => {
    await session.invalidate(id);
    await Promise.all([loadFacts(), refreshHistory(), refreshSessions()]);
  }, [loadFacts, refreshHistory, refreshSessions, session]);

  const getImpact = useCallback(async (id: string) => {
    try {
      return await session.getImpact(id);
    } catch (err) {
      console.error('Impact analysis failed:', err);
      return EMPTY_IMPACT;
    }
  }, [session]);

  const getWhy = useCallback(async (id: string) => {
    try {
      const tree = await session.getWhy(id);
      return Array.from(flattenExplanation(tree));
    } catch (err) {
      console.error('Provenance analysis failed:', err);
      return [];
    }
  }, [session]);

  return {
    apiKey,
    connect,
    disconnect,
    authRequired,
    sessionId,
    setSessionId,
    sessions,
    activeSessionInfo,
    facts,
    history,
    loading,
    error,
    invalidateFact,
    getImpact,
    getWhy,
    refreshHistory,
    refreshSessions,
  };
}
