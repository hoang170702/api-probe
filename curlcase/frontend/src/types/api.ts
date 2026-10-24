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
