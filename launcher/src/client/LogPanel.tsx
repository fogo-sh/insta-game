import { h } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import { pollLogs } from "./api.js";

interface LogPanelProps {
  game: string;
  passphrase: string;
}

export function LogPanel({ game, passphrase }: LogPanelProps) {
  const [lines, setLines] = useState<string[]>([`[connecting to ${game} logs...]`]);
  const [logMode, setLogMode] = useState<"sse" | "poll" | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const cursorRef = useRef<string | null>(null);
  const activeRef = useRef(true);

  // Auto-scroll when lines change
  useEffect(() => {
    const el = panelRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [lines]);

  // Detect log mode from server then start streaming
  useEffect(() => {
    activeRef.current = true;

    fetch(`/logs?game=${encodeURIComponent(game)}&token=${encodeURIComponent(passphrase)}`, {
      method: "HEAD",
    }).then(res => {
      if (!activeRef.current) return;
      const mode = res.headers.get("X-Log-Mode") === "poll" ? "poll" : "sse";
      setLogMode(mode);
    }).catch(() => {
      if (activeRef.current) setLines(l => [...l, "[failed to connect]"]);
    });

    return () => { activeRef.current = false; };
  }, [game, passphrase]);

  // SSE mode
  useEffect(() => {
    if (logMode !== "sse") return;
    const url = `/logs?game=${encodeURIComponent(game)}&token=${encodeURIComponent(passphrase)}`;
    const es = new EventSource(url);

    es.addEventListener("log", (e: MessageEvent) => {
      if (!activeRef.current) return;
      setLines(l => [...l, e.data as string]);
    });

    es.onerror = () => {
      if (activeRef.current) setLines(l => [...l, "[log stream disconnected]"]);
      es.close();
    };

    return () => { es.close(); };
  }, [logMode, game, passphrase]);

  // Poll mode
  useEffect(() => {
    if (logMode !== "poll") return;
    let stopped = false;

    async function doPoll() {
      while (!stopped && activeRef.current) {
        try {
          const result = await pollLogs(game, passphrase, cursorRef.current);
          if (result.lines.length > 0) {
            setLines(l => [...l, ...result.lines]);
          }
          if (result.cursor) cursorRef.current = result.cursor;
        } catch (err) {
          if (!stopped) setLines(l => [...l, `[poll error: ${err instanceof Error ? err.message : String(err)}]`]);
        }
        await new Promise(r => setTimeout(r, 5000));
      }
    }

    void doPoll();
    return () => { stopped = true; };
  }, [logMode, game, passphrase]);

  return (
    <div class="log-panel" ref={panelRef}>
      {lines.map((line, i) => (
        <div
          key={i}
          class="log-line"
          dangerouslySetInnerHTML={{ __html: line }}
        />
      ))}
    </div>
  );
}
