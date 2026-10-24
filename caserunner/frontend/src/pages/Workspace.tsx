import { useEffect, useRef, useState } from 'react'
import { Alert, Card, Flex, Segmented, Tag, Typography } from 'antd'
import { getHealth } from '../api/health'
import { parseCurl } from '../api/parse'
import { tryPrepare } from '../api/prepare'
import type { Capture, FieldRule, ParseResponse, StepResult } from '../types/api'
import { useI18n } from '../i18n/context'
import type { Lang } from '../i18n/messages'
import CurlInput from '../components/CurlInput'
import RequestSummary from '../components/RequestSummary'
import FieldRuleTable from '../components/FieldRuleTable'
import PrepareSteps, { type PrepareStepState } from '../components/PrepareSteps'

type Health = { state: 'checking' } | { state: 'ok'; version: string } | { state: 'down'; message: string }

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/**
 * The single API Case Runner page (BRD-001 §16): optional token steps, the
 * curl panel, then the configuration below it once the curl is analyzed.
 * The case table and results panel come in later steps.
 */
export default function Workspace() {
  const { t, lang, setLang } = useI18n()
  const [health, setHealth] = useState<Health>({ state: 'checking' })
  const [curl, setCurl] = useState('')
  const [analyzing, setAnalyzing] = useState(false)
  const [parseError, setParseError] = useState<string>()
  const [parsed, setParsed] = useState<ParseResponse>()
  const [rules, setRules] = useState<Record<string, FieldRule>>({})

  const [steps, setSteps] = useState<PrepareStepState[]>([])
  const [injectToken, setInjectToken] = useState(true)
  const [trying, setTrying] = useState(false)
  const [tryError, setTryError] = useState<string>()
  const [stepResults, setStepResults] = useState<StepResult[]>()
  const nextStepId = useRef(1)

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
      setParseError(errorMessage(err))
    } finally {
      setAnalyzing(false)
    }
  }

  // Any edit to the steps makes the last Try result stale.
  function updateStep(id: number, patch: Partial<PrepareStepState>) {
    setSteps((prev) => prev.map((s) => (s.id === id ? { ...s, ...patch } : s)))
    setStepResults(undefined)
  }

  function addStep() {
    const id = nextStepId.current++
    // The first step captures the token (§11.3); later steps may capture nothing.
    const captures: Capture[] = steps.length === 0 ? [{ var: 'token', path: '' }] : []
    setSteps((prev) => [...prev, { id, curl: '', analyzing: false, captures }])
    setStepResults(undefined)
  }

  function removeStep(id: number) {
    setSteps((prev) => prev.filter((s) => s.id !== id))
    setStepResults(undefined)
  }

  async function analyzeStep(id: number) {
    const step = steps.find((s) => s.id === id)
    if (!step) return
    updateStep(id, { analyzing: true, parseError: undefined })
    try {
      updateStep(id, { parsed: await parseCurl(step.curl), analyzing: false })
    } catch (err) {
      updateStep(id, { parsed: undefined, parseError: errorMessage(err), analyzing: false })
    }
  }

  async function runTry() {
    setTrying(true)
    setTryError(undefined)
    try {
      const res = await tryPrepare(
        steps.map((s) => ({
          template: s.parsed!.template,
          captures: s.captures.filter((c) => c.var !== '' || c.path !== ''), // a blank row is not a capture
        })),
      )
      setStepResults(res.steps)
    } catch (err) {
      setStepResults(undefined)
      setTryError(errorMessage(err))
    } finally {
      setTrying(false)
    }
  }

  const knownVars = steps.flatMap((s) => s.captures.map((c) => c.var)).filter((v) => v !== '')
  const tokenInjected = steps.length > 0 && injectToken

  return (
    <Flex vertical gap={24}>
      <Flex justify="space-between" align="center" wrap gap={12}>
        <Typography.Title level={2} style={{ margin: 0 }}>
          {t('app.title')}
        </Typography.Title>
        <Flex gap={12} align="center">
          <Segmented<Lang>
            aria-label={t('lang.switch')}
            value={lang}
            onChange={setLang}
            options={[
              { label: 'VI', value: 'vi' },
              { label: 'EN', value: 'en' },
            ]}
          />
          <HealthTag health={health} />
        </Flex>
      </Flex>

      {health.state === 'down' && <Alert type="error" showIcon title={t('health.unreachable')} description={health.message} />}

      <Card title={t('prepare.card')}>
        <PrepareSteps
          steps={steps}
          onAdd={addStep}
          onRemove={removeStep}
          onCurlChange={(id, value) => updateStep(id, { curl: value })}
          onAnalyze={analyzeStep}
          onCapturesChange={(id, captures) => updateStep(id, { captures })}
          injectToken={injectToken}
          onInjectTokenChange={setInjectToken}
          onTry={runTry}
          trying={trying}
          tryError={tryError}
          results={stepResults}
        />
      </Card>

      <Card title={t('curl.card')}>
        <CurlInput value={curl} onChange={setCurl} onAnalyze={analyze} loading={analyzing} error={parseError} />
      </Card>

      {parsed && (
        <>
          <Card title={t('request.card')}>
            <RequestSummary
              template={parsed.template}
              warnings={parsed.warnings}
              injectToken={tokenInjected}
              knownVars={knownVars}
            />
          </Card>
          {parsed.template.bodyKind === 'json' && (
            <Card title={t('fields.card')}>
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
  const { t } = useI18n()
  switch (health.state) {
    case 'checking':
      return <Tag>{t('health.checking')}</Tag>
    case 'ok':
      return <Tag color="success">{t('health.ok', { version: health.version })}</Tag>
    default:
      return <Tag color="error">{t('health.down')}</Tag>
  }
}
