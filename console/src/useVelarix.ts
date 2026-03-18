import { useState, useEffect, useCallback, useMemo } from 'react';
import { VelarixClient } from './lib/client';
import type { Fact, ChangeEvent, JournalEntry, SessionInfo } from './lib/types';

export function useVelarix(url: string = 'http://localhost:8080') {
  const [apiKey, setApiKey] = useState(() => localStorage.getItem('velarix_api_key') || '');
  const [sessionId, setSessionId] = useState('default-session');
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [facts, setFacts] = useState<Record<string, Fact>>({});
  const [history, setHistory] = useState<JournalEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [authRequired, setAuthRequired] = useState(false);

  const client = useMemo(() => new VelarixClient(url, apiKey), [url, apiKey]);
  const session = useMemo(() => client.session(sessionId), [client, sessionId]);

  const activeSessionInfo = useMemo(() => 
    sessions.find(s => s.id === sessionId), 
  [sessions, sessionId]);

  const refreshSessions = useCallback(async () => {
    try {
      const s = await client.getSessions();
      setSessions(Array.isArray(s) ? s : []);
      setAuthRequired(false);
      setError(null);
    } catch (err: any) {
      console.error("Failed to load sessions:", err);
      if (err.message.includes("401") || err.message.includes("unauthorized")) {
        setAuthRequired(true);
      } else {
        setError(err.message || "Failed to connect to backend");
      }
    }
  }, [client]);

  const refreshHistory = useCallback(async () => {
    try {
      const h = await session.getHistory();
      setHistory(Array.isArray(h) ? h : []);
    } catch (err: any) {
      console.error("Failed to load history:", err);
    }
  }, [session]);

  const loadFacts = useCallback(async () => {
    try {
      setLoading(true);
      const data = await session.getFacts();
      const factMap: Record<string, Fact> = {};
      if (Array.isArray(data)) {
        data.forEach(f => {
          factMap[f.ID] = f;
        });
      }
      setFacts(factMap);
      setError(null);
    } catch (err: any) {
      console.error("Failed to load facts:", err);
      if (err.message.includes("401") || err.message.includes("unauthorized")) {
        setAuthRequired(true);
      } else {
        setError(err.message || "Failed to load facts");
      }
    } finally {
      setLoading(false);
    }
  }, [session]);

  const connect = useCallback((key: string) => {
    localStorage.setItem('velarix_api_key', key);
    setApiKey(key);
    setAuthRequired(false);
    setError(null);
  }, []);

  // Initial load and session change
  useEffect(() => {
    refreshSessions();
  }, [refreshSessions]);

  useEffect(() => {
    loadFacts();
    refreshHistory();
  }, [loadFacts, refreshHistory, sessionId]);

  // Subscribe to real-time collapses
  useEffect(() => {
    let unsubscribe: (() => void) | null = null;
    try {
      unsubscribe = session.listen((event: ChangeEvent) => {
        setFacts(prev => {
          const target = prev[event.fact_id];
          if (!target) return prev; 
          return {
            ...prev,
            [event.fact_id]: {
              ...target,
              resolved_status: event.status
            }
          };
        });
        refreshHistory();
        refreshSessions();
      });
    } catch (err) {
      console.error("SSE Connection failed:", err);
    }

    return () => {
      if (unsubscribe) unsubscribe();
    };
  }, [session, refreshHistory, refreshSessions]);

  const invalidateFact = useCallback(async (id: string) => {
    try {
      await session.invalidate(id);
      refreshSessions();
    } catch (err: any) {
      console.error("Failed to invalidate:", err);
      alert("Failed to invalidate: " + err.message);
    }
  }, [session, refreshSessions]);

  const getImpact = useCallback(async (id: string) => {
    try {
      const res = await session.getImpact(id);
      return Array.isArray(res) ? res : [];
    } catch (err) {
      console.error("Impact analysis failed:", err);
      return [];
    }
  }, [session]);

  const getWhy = useCallback(async (id: string) => {
    try {
      const res = await session.getWhy(id);
      return Array.isArray(res) ? res : [];
    } catch (err) {
      console.error("Provenance analysis failed:", err);
      return [];
    }
  }, [session]);

  return { 
    apiKey,
    connect,
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
    refreshSessions
  };
}
