import { useEffect, useState } from 'react'
import { Alert, Card, Flex, Tag, Typography } from 'antd'
import { getHealth } from '../api/health'
import { parseCurl } from '../api/parse'
import { ApiError } from '../api/client'
import type { FieldRule, ParseResponse } from '../types/api'
import CurlInput from '../components/CurlInput'
import RequestSummary from '../components/RequestSummary'
import FieldRuleTable from '../components/FieldRuleTable'

type Health = { state: 'checking' } | { state: 'ok'; version: string } | { state: 'down'; message: string }

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/**
 * The single API Case Runner page (BRD-001 §16): the curl panel, then the
 * configuration panel below it once the curl is analyzed. The case table
 * and results panel come in later steps.
 */
export default function Workspace() {
  const [health, setHealth] = useState<Health>({ state: 'checking' })
  const [curl, setCurl] = useState('')
  const [analyzing, setAnalyzing] = useState(false)
  const [parseError, setParseError] = useState<string>()
  const [parsed, setParsed] = useState<ParseResponse>()
  const [rules, setRules] = useState<Record<string, FieldRule>>({})

  useEffect(() => {
    const ctrl = new AbortController()
    getHealth(ctrl.signal)
      .then((h) => setHealth({ state: 'ok', version: h.version }))
      .catch((err: unknown) => {
        if (!ctrl.signal.aborted) setHealth({ state: 'down', message: errorMessage(err) })
      })
    return () => ctrl.abort()
  }, [])

  async function analyze() {
    setAnalyzing(true)
    setParseError(undefined)
    try {
      const res = await parseCurl(curl)
      setParsed(res)
      // Default Required (D3); a path that still exists keeps the rule it had.
      setRules((prev) => Object.fromEntries(res.fields.map((f) => [f.path, prev[f.path] ?? 'REQUIRED'])))
    } catch (err) {
      setParsed(undefined)
      setParseError(err instanceof ApiError ? err.message : errorMessage(err))
    } finally {
      setAnalyzing(false)
    }
  }

  return (
    <Flex vertical gap={24}>
      <Flex justify="space-between" align="center">
        <Typography.Title level={2} style={{ margin: 0 }}>
          API Case Runner
        </Typography.Title>
        <HealthTag health={health} />
      </Flex>

      {health.state === 'down' && <Alert type="error" showIcon title="Backend unreachable" description={health.message} />}

      <Card title="1. Curl">
        <CurlInput value={curl} onChange={setCurl} onAnalyze={analyze} loading={analyzing} error={parseError} />
      </Card>

      {parsed && (
        <>
          <Card title="2. Request">
            <RequestSummary template={parsed.template} warnings={parsed.warnings} />
          </Card>
          {parsed.template.bodyKind === 'json' && (
            <Card title="3. Field rules">
              <FieldRuleTable
                fields={parsed.fields}
                rules={rules}
                onRuleChange={(path, rule) => setRules((prev) => ({ ...prev, [path]: rule }))}
                onSetAll={(rule) => setRules(Object.fromEntries(parsed.fields.map((f) => [f.path, rule])))}
              />
            </Card>
          )}
        </>
      )}
    </Flex>
  )
}

function HealthTag({ health }: { health: Health }) {
  switch (health.state) {
    case 'checking':
      return <Tag>Checking backend…</Tag>
    case 'ok':
      return <Tag color="success">BE OK · {health.version}</Tag>
    default:
      return <Tag color="error">BE down</Tag>
  }
}
