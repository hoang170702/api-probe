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
