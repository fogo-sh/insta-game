function ts(): string {
  return new Date().toISOString();
}

export const log = {
  info: (msg: string) => console.log(`${ts()} INFO  ${msg}`),
  warn: (msg: string) => console.warn(`${ts()} WARN  ${msg}`),
  error: (msg: string, err?: unknown) => {
    const detail = err instanceof Error ? ` — ${err.message}` : err != null ? ` — ${String(err)}` : "";
    console.error(`${ts()} ERROR ${msg}${detail}`);
  },
};
