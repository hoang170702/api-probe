import type { RequestTemplate } from '../types/api'

const PLACEHOLDER = /\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}/g

/** Variable names used as {{name}} in the URL, header values and body, once each (§11.3). */
export function placeholders(t: RequestTemplate): string[] {
  const names: string[] = []
  for (const text of [t.url, ...t.headers.map((h) => h.value), t.bodyRaw]) {
    for (const m of text.matchAll(PLACEHOLDER)) {
      if (!names.includes(m[1])) names.push(m[1])
    }
  }
  return names
}
