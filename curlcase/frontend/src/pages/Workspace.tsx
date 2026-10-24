import { useEffect, useState } from 'react'
import { Alert, Card, Space, Typography } from 'antd'
import { getHealth } from '../api/health'
import { ApiError } from '../api/client'

type Health =
  | { state: 'checking' }
  | { state: 'ok'; version: string }
  | { state: 'down'; message: string }

/**
 * The single CurlCase page (BRD-001 §16). Step 0 only checks the backend;
 * the curl, configuration and results panels arrive in later steps.
 */
export default function Workspace() {
  const [health, setHealth] = useState<Health>({ state: 'checking' })

  useEffect(() => {
    const ctrl = new AbortController()
    getHealth(ctrl.signal)
      .then((h) => setHealth({ state: 'ok', version: h.version }))
      .catch((err: unknown) => {
        if (ctrl.signal.aborted) return
        setHealth({ state: 'down', message: err instanceof ApiError ? err.message : String(err) })
      })
    return () => ctrl.abort()
  }, [])

  return (
    <Space orientation="vertical" size="large" style={{ width: '100%' }}>
      <Typography.Title level={2} style={{ margin: 0 }}>
        CurlCase
      </Typography.Title>
      <Card title="Backend">
        {health.state === 'checking' && <Alert type="info" showIcon title="Checking the backend…" />}
        {health.state === 'ok' && <Alert type="success" showIcon title={`BE OK (version ${health.version})`} />}
        {health.state === 'down' && <Alert type="error" showIcon title="BE unreachable" description={health.message} />}
      </Card>
    </Space>
  )
}
