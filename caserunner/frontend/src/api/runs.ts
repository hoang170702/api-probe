import type { CaseResult, RunDone, RunSnapshot, StartRunRequest, StartRunResponse } from '../types/api'
import { requestJson } from './client'

export function startRun(req: StartRunRequest): Promise<StartRunResponse> {
  return requestJson<StartRunResponse>('/runs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export function getRun(id: string): Promise<RunSnapshot> {
  return requestJson<RunSnapshot>(`/runs/${encodeURIComponent(id)}`)
}

export function cancelRun(id: string): Promise<unknown> {
  return requestJson(`/runs/${encodeURIComponent(id)}/cancel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
  })
}

export interface RunWatcher {
  onSnapshot: (s: RunSnapshot) => void
  onCase: (c: CaseResult) => void
  onDone: (d: RunDone) => void
  onError: (message: string) => void
}

/**
 * Follows a run over SSE (BRD-001 §15.5). When the stream breaks before the
 * run is done, it falls back to polling the snapshot every 2 seconds.
 * Returns a function that stops watching.
 */
export function watchRun(id: string, w: RunWatcher): () => void {
  let stopped = false
  let poll: number | undefined
  const es = new EventSource(`/api/runs/${encodeURIComponent(id)}/events`)
  const finish = (d: RunDone) => {
    stopped = true
    es.close()
    window.clearInterval(poll)
    w.onDone(d)
  }
  es.addEventListener('snapshot', (e) => w.onSnapshot(JSON.parse((e as MessageEvent).data)))
  es.addEventListener('case', (e) => w.onCase(JSON.parse((e as MessageEvent).data)))
  // "done" carries only the summary: cases cancelled before they were sent
  // have no event of their own, so the final state is read once more.
  es.addEventListener('done', (e) => {
    const d: RunDone = JSON.parse((e as MessageEvent).data)
    stopped = true
    es.close()
    getRun(id)
      .then((s) => w.onSnapshot(s))
      .catch(() => undefined)
      .finally(() => finish(d))
  })
  es.onerror = () => {
    if (stopped) return
    es.close()
    const tick = async () => {
      try {
        const s = await getRun(id)
        w.onSnapshot(s)
        if (s.status !== 'RUNNING') finish({ runId: s.runId, status: s.status, summary: s.summary })
      } catch (err) {
        stopped = true
        window.clearInterval(poll)
        w.onError(err instanceof Error ? err.message : String(err))
      }
    }
    void tick()
    poll = window.setInterval(() => void tick(), 2000)
  }
  return () => {
    stopped = true
    es.close()
    window.clearInterval(poll)
  }
}
