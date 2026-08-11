import { useCallback, useEffect, useRef, useState, type UIEvent } from "react";
import { websocketUrl } from "../lib/api";

export function useLogStream(authenticated: boolean, selectedName: string) {
  const [logs, setLogs] = useState<string[]>([]);
  const outputRef = useRef<HTMLPreElement>(null);
  const followingRef = useRef(true);
  const [following, setFollowing] = useState(true);

  useEffect(() => {
    if (!authenticated || !selectedName) return;

    const clearTimer = window.setTimeout(() => {
      setLogs([]);
      followingRef.current = true;
      setFollowing(true);
    }, 0);
    let socket: WebSocket | null = null;
    let retryTimer: number | undefined;
    let disposed = false;

    const connect = () => {
      socket = new WebSocket(websocketUrl(`/apps/${encodeURIComponent(selectedName)}/logs?follow=true&tail=100`));
      socket.onmessage = (event) => setLogs((current) => [...current, String(event.data)].slice(-300));
      socket.onclose = () => {
        if (!disposed) retryTimer = window.setTimeout(connect, 2000);
      };
      socket.onerror = () => socket?.close();
    };

    connect();
    return () => {
      disposed = true;
      window.clearTimeout(clearTimer);
      if (retryTimer) window.clearTimeout(retryTimer);
      socket?.close();
    };
  }, [authenticated, selectedName]);

  useEffect(() => {
    const output = outputRef.current;
    if (output && followingRef.current) output.scrollTop = output.scrollHeight;
  }, [logs]);

  const handleScroll = useCallback((event: UIEvent<HTMLPreElement>) => {
    const output = event.currentTarget;
    const atBottom = output.scrollHeight - output.scrollTop - output.clientHeight <= 24;
    followingRef.current = atBottom;
    setFollowing(atBottom);
  }, []);

  const resumeFollowing = useCallback(() => {
    followingRef.current = true;
    setFollowing(true);
    const output = outputRef.current;
    if (output) output.scrollTop = output.scrollHeight;
  }, []);

  const clearLogs = () => setLogs([]);
  return { logs, outputRef, following, handleScroll, resumeFollowing, clearLogs };
}
