import type { Capture, FieldRule, RunOptionsValue } from '../types/api'

/**
 * A saved configuration (BRD-001 D11): everything needed to set the page up
 * again. Captured values are never saved; the token steps fetch them anew.
 */
export interface ConfigFile {
  version: 1
  prepare: { steps: { curl: string; captures: Capture[] }[]; injectToken: boolean }
  curl: string
  rules: Record<string, FieldRule>
  cases: { name: string; values: Record<string, string>; expect: string }[]
  options: RunOptionsValue
}

export function downloadConfig(cfg: ConfigFile): void {
  const blob = new Blob([JSON.stringify(cfg, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `caserunner-config-${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
}

const isRecord = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v)
const isStringMap = (v: unknown): v is Record<string, string> => isRecord(v) && Object.values(v).every((x) => typeof x === 'string')

/** Reads a config file, throwing an Error that names the first problem. */
export function parseConfig(text: string): ConfigFile {
  let v: unknown
  try {
    v = JSON.parse(text)
  } catch {
    throw new Error('The file is not JSON')
  }
  if (!isRecord(v) || v.version !== 1) throw new Error('Not an API Case Runner config (version 1)')
  if (typeof v.curl !== 'string') throw new Error('curl is missing')
  const prepare = isRecord(v.prepare) ? v.prepare : { steps: [], injectToken: true }
  const steps = Array.isArray(prepare.steps) ? prepare.steps : []
  if (!steps.every((s) => isRecord(s) && typeof s.curl === 'string' && Array.isArray(s.captures))) throw new Error('prepare.steps is invalid')
  if (!Array.isArray(v.cases) || !v.cases.every((c) => isRecord(c) && isStringMap(c.values) && typeof c.expect === 'string')) {
    throw new Error('cases is invalid')
  }
  const rules = isRecord(v.rules) ? v.rules : {}
  const options = isRecord(v.options) ? v.options : {}
  return {
    version: 1,
    prepare: {
      steps: steps.map((s) => ({
        curl: s.curl as string,
        captures: (s.captures as unknown[]).filter(isRecord).map((c) => ({ var: String(c.var ?? ''), path: String(c.path ?? '') })),
      })),
      injectToken: prepare.injectToken !== false,
    },
    curl: v.curl,
    rules: Object.fromEntries(Object.entries(rules).filter(([, r]) => r === 'REQUIRED' || r === 'UNREQUIRED')) as Record<string, FieldRule>,
    cases: (v.cases as Record<string, unknown>[]).map((c) => ({
      name: typeof c.name === 'string' ? c.name : '',
      values: c.values as Record<string, string>,
      expect: c.expect as string,
    })),
    options: {
      concurrency: typeof options.concurrency === 'number' ? options.concurrency : 5,
      timeoutSec: typeof options.timeoutSec === 'number' ? options.timeoutSec : 30,
    },
  }
}
