import type { HealthResponse } from '../types/api'
import { requestJson } from './client'

export function getHealth(signal?: AbortSignal): Promise<HealthResponse> {
  return requestJson<HealthResponse>('/health', { signal })
}
