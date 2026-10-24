import type { ParseResponse } from '../types/api'
import { requestJson } from './client'

export function parseCurl(curl: string): Promise<ParseResponse> {
  return requestJson<ParseResponse>('/parse', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ curl }),
  })
}
