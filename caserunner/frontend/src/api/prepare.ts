import type { PrepareStepInput, TryPrepareResponse } from '../types/api'
import { requestJson } from './client'

export function tryPrepare(steps: PrepareStepInput[]): Promise<TryPrepareResponse> {
  return requestJson<TryPrepareResponse>('/prepare/try', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ steps }),
  })
}
