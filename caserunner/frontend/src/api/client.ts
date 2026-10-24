import type { ApiErrorBody } from '../types/api'

/** An error the backend reported in its common error format, or a transport failure. */
export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

function isApiErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== 'object' || value === null || !('error' in value)) return false
  const { error } = value as { error: unknown }
  return typeof error === 'object' && error !== null && 'code' in error && 'message' in error
}

/** Calls a backend endpoint under /api and returns its JSON body. */
export async function requestJson<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      ...init,
      headers: { Accept: 'application/json', ...init?.headers },
    })
  } catch {
    throw new ApiError('NETWORK', 'Cannot reach the API Case Runner backend', 0)
  }

  const body: unknown = await res.json().catch(() => undefined)
  if (!res.ok) {
    if (isApiErrorBody(body)) throw new ApiError(body.error.code, body.error.message, res.status)
    throw new ApiError('HTTP_ERROR', `Backend returned HTTP ${res.status}`, res.status)
  }
  return body as T
}
