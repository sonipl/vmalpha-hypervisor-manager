import { useEffect, useRef, useCallback, useState } from 'react';
import type { WSEvent } from '@/types';

export function useWebSocket(onEvent?: (event: WSEvent) => void) {
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>();

  const connect = useCallback(() => {
    const token = localStorage.getItem('nova_token');
    if (!token) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/api/v1/ws?token=${token}`);

    ws.onopen = () => {
      setConnected(true);
      console.log('[WS] Connected');
    };

    ws.onmessage = (event) => {
      try {
        const data: WSEvent = JSON.parse(event.data);
        onEvent?.(data);
      } catch (err) {
        console.error('[WS] Parse error:', err);
      }
    };

    ws.onclose = () => {
      setConnected(false);
      console.log('[WS] Disconnected, reconnecting in 5s...');
      reconnectTimer.current = setTimeout(connect, 5000);
    };

    ws.onerror = (err) => {
      console.error('[WS] Error:', err);
      ws.close();
    };

    wsRef.current = ws;
  }, [onEvent]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { connected };
}
