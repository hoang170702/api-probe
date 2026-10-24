// Types matching the backend API (BRD-001 §15).

/** Common error body returned by every failing endpoint (§15.1). */
export interface ApiErrorBody {
  error: {
    code: string
    message: string
  }
}

export interface HealthResponse {
  status: 'ok'
  service: string
  version: string
}

export type BodyKind = 'none' | 'json' | 'unsupported'

export interface Header {
  name: string
  value: string
}

export interface RequestTemplate {
  method: string
  url: string
  headers: Header[]
  bodyRaw: string
  bodyKind: BodyKind
  insecure: boolean
}

export type FieldType = 'string' | 'number' | 'boolean' | 'null' | 'object' | 'array'

export interface Field {
  path: string
  type: FieldType
  sampleValue: string
}

export interface ParseResponse {
  template: RequestTemplate
  fields: Field[]
  warnings: string[]
}

export type FieldRule = 'REQUIRED' | 'UNREQUIRED'

/** One capture of a prepare step: an empty path searches for a token (§11.3). */
export interface Capture {
  var: string
  path: string
}

export interface PrepareStepInput {
  template: RequestTemplate
  captures: Capture[]
}

export type StepStatus = 'OK' | 'FAILED' | 'SKIPPED'

export interface StepResult {
  status: StepStatus
  httpStatus?: number
  durationMs: number
  reason: string
  errorType?: string
  captured: (Capture & { value: string })[]
  responseBody: string
  truncated: boolean
}

export interface TryPrepareResponse {
  steps: StepResult[]
  vars: Record<string, string>
}

/** One row of the case table (BRD-001 §8): values are typed text, keyed by field path. */
export interface CaseRow {
  id: number
  name: string
  values: Record<string, string>
  expect: string
}

export interface RunOptionsValue {
  concurrency: number
  timeoutSec: number
}
