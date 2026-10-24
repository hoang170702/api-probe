import { useEffect, useRef, useState } from 'react'
import { cancelRun, startRun, watchRun } from '../api/runs'
import { Alert, App, Button, Card, Flex, Segmented, Tag, Typography } from 'antd'
import { getHealth } from '../api/health'
import { parseCurl } from '../api/parse'
import { tryPrepare } from '../api/prepare'
import type { Capture, CaseResult, CaseRow, FieldRule, ParseResponse, RunOptionsValue, RunStatus, StartRunRequest, StepResult } from '../types/api'
import { useI18n } from '../i18n/context'
import type { Lang } from '../i18n/messages'
import CurlInput from '../components/CurlInput'
import RequestSummary from '../components/RequestSummary'
import FieldRuleTable from '../components/FieldRuleTable'
import PrepareSteps, { type PrepareStepState } from '../components/PrepareSteps'
import CaseTable from '../components/CaseTable'
import RunOptions from '../components/RunOptions'
import ResultView from '../components/ResultView'
import { remapRows, rowProblems, sampleRow } from '../lib/cases'
import { downloadConfig, parseConfig, type ConfigFile } from '../lib/config-file'

type Health = { state: 'checking' } | { state: 'ok'; version: string } | { state: 'down'; message: string }

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/**
 * The single API Case Runner page (BRD-001 §16): optional token steps, the
 * curl panel, then the configuration below it once the curl is analyzed:
 * field rules, the case table and the run options; the results appear
 * below once a run starts.
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

  const { modal, message } = App.useApp()
  const [rows, setRows] = useState<CaseRow[]>([])
  const [options, setOptions] = useState<RunOptionsValue>({ concurrency: 5, timeoutSec: 30 })
  const nextRowId = useRef(1)
  const fileInput = useRef<HTMLInputElement>(null)

  const [runState, setRunState] = useState<{ id: string; status: RunStatus; results: CaseResult[] }>()
  const [starting, setStarting] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const stopWatching = useRef<() => void>(undefined)
  useEffect(() => () => stopWatching.current?.(), [])

  useEffect(() => {
    const ctrl = new AbortController()
    getHealth(ctrl.signal)
      .then((h) => setHealth({ state: 'ok', version: h.version }))
      .catch((err: unknown) => {
        if (!ctrl.signal.aborted) setHealth({ state: 'down', message: errorMessage(err) })
      })
    return () => ctrl.abort()
  }, [])

  // Shows a parsed curl: rules default to Required (D3) and the rows follow
  // its fields (§8.2), asking first when typed values would be lost.
  async function applyParsed(res: ParseResponse, prevRules: Record<string, FieldRule>, keepRows: CaseRow[]): Promise<void> {
    const { rows: next, lost } = remapRows(keepRows, res.fields)
    if (lost > 0) {
      const ok = await modal.confirm({
        title: t('config.lostTitle'),
        content: t('config.lostBody', { n: lost }),
        okText: t('config.continue'),
        cancelText: t('config.cancel'),
      })
      if (!ok) return
    }
    setParsed(res)
    setRules(Object.fromEntries(res.fields.map((f) => [f.path, prevRules[f.path] ?? 'REQUIRED'])))
    setRows(next.length > 0 ? next : [sampleRow(nextRowId.current++, res.fields)])
  }

  function addRow() {
    if (parsed) setRows((prev) => [...prev, sampleRow(nextRowId.current++, parsed.fields)])
  }

  function duplicateRow(id: number) {
    setRows((prev) => prev.flatMap((r) => (r.id === id ? [r, { ...r, id: nextRowId.current++, values: { ...r.values } }] : [r])))
  }

  async function analyze() {
    setAnalyzing(true)
    setParseError(undefined)
    try {
      await applyParsed(await parseCurl(curl), rules, rows)
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

  function currentConfig(): ConfigFile {
    return {
      version: 1,
      prepare: { steps: steps.map((s) => ({ curl: s.curl, captures: s.captures })), injectToken },
      curl,
      rules,
      cases: rows.map(({ name, values, expect }) => ({ name, values, expect })),
      options,
    }
  }

  // Restores a saved configuration and analyzes its curls again (D11).
  async function importConfig(file: File) {
    try {
      const cfg = parseConfig(await file.text())
      const restored: PrepareStepState[] = []
      for (const s of cfg.prepare.steps) {
        const step: PrepareStepState = { id: nextStepId.current++, curl: s.curl, analyzing: false, captures: s.captures }
        try {
          step.parsed = await parseCurl(s.curl)
        } catch (err) {
          step.parseError = errorMessage(err)
        }
        restored.push(step)
      }
      setSteps(restored)
      setStepResults(undefined)
      setInjectToken(cfg.prepare.injectToken)
      setOptions(cfg.options)
      setCurl(cfg.curl)
      setParseError(undefined)
      const res = await parseCurl(cfg.curl)
      const imported = cfg.cases.map((c) => ({ id: nextRowId.current++, ...c }))
      await applyParsed(res, cfg.rules, imported)
    } catch (err) {
      message.error(`${t('config.importFailed')}: ${errorMessage(err)}`)
    }
  }

  function runRequest(p: ParseResponse): StartRunRequest {
    return {
      template: p.template,
      fields: p.fields.map((f) => ({ path: f.path, type: f.type, rule: rules[f.path] ?? 'REQUIRED' })),
      cases: rows.map((r, i) => ({ no: i + 1, name: r.name, values: r.values, expect: r.expect })),
      ...(steps.length > 0 && {
        prepare: {
          steps: steps.map((s) => ({ template: s.parsed!.template, captures: s.captures.filter((c) => c.var !== '' || c.path !== '') })),
          injectToken,
        },
      }),
      options: { concurrency: options.concurrency, timeoutMs: options.timeoutSec * 1000 },
    }
  }

  // Starts a run and follows it over SSE (BRD-001 §15.5); the backend validates again.
  async function run() {
    if (!parsed) return
    const blocked = rows.filter((r) => rowProblems(r, parsed.fields, rules).length > 0).length
    if (blocked > 0) {
      const go = await modal.confirm({
        title: t('run.blockedTitle', { n: blocked }),
        content: t('run.blockedBody'),
        okText: t('run.button'),
        cancelText: t('config.cancel'),
      })
      if (!go) return
    }
    setStarting(true)
    try {
      const started = await startRun(runRequest(parsed))
      stopWatching.current?.()
      setRunState({ id: started.runId, status: 'RUNNING', results: [] })
      stopWatching.current = watchRun(started.runId, {
        onSnapshot: (snap) =>
          setRunState((prev) => (prev && prev.id !== snap.runId ? prev : { id: snap.runId, status: snap.status, results: snap.results })),
        onCase: (c) =>
          setRunState((prev) => {
            if (!prev || prev.id !== started.runId) return prev
            const results = [...prev.results]
            results[c.caseIndex] = c
            return { ...prev, results }
          }),
        onDone: (d) => setRunState((prev) => (prev && prev.id === d.runId ? { ...prev, status: d.status } : prev)),
        // The run can no longer be followed (expired, or the tool stopped):
        // drop it so Run is enabled again.
        onError: (msg) => {
          message.error(`${t('results.lost')}: ${msg}`)
          setRunState((prev) => (prev && prev.id === started.runId ? undefined : prev))
        },
      })
    } catch (err) {
      message.error(`${t('run.failed')}: ${errorMessage(err)}`)
    } finally {
      setStarting(false)
    }
  }

  async function cancel() {
    if (!runState) return
    setCancelling(true)
    try {
      await cancelRun(runState.id)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setCancelling(false)
    }
  }

  const stepsReady = steps.every((s) => s.parsed)
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

      <Card
        title={t('curl.card')}
        extra={
          <>
            <input
              ref={fileInput}
              type="file"
              accept="application/json,.json"
              hidden
              onChange={(e) => {
                const f = e.target.files?.[0]
                e.target.value = ''
                if (f) void importConfig(f)
              }}
            />
            <Button size="small" onClick={() => fileInput.current?.click()}>
              {t('config.import')}
            </Button>
          </>
        }
      >
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
          {parsed.template.bodyKind === 'json' && (
            <Card
              title={t('cases.card')}
              extra={
                <Button size="small" onClick={() => downloadConfig(currentConfig())}>
                  {t('config.export')}
                </Button>
              }
            >
              <Flex vertical gap={16}>
                <CaseTable
                  fields={parsed.fields}
                  rules={rules}
                  rows={rows}
                  onChange={setRows}
                  onAdd={addRow}
                  onDuplicate={duplicateRow}
                />
                <RunOptions value={options} onChange={setOptions} sequential={steps.length > 0} />
                <Flex gap={12} align="center">
                  <Button
                    type="primary"
                    onClick={() => void run()}
                    loading={starting}
                    disabled={rows.length === 0 || !stepsReady || runState?.status === 'RUNNING'}
                  >
                    {t('run.button')}
                  </Button>
                  {!stepsReady && <Typography.Text type="secondary">{t('prepare.needAnalyze')}</Typography.Text>}
                </Flex>
              </Flex>
            </Card>
          )}
          {runState && (
            <Card title={t('results.card')}>
              <ResultView
                status={runState.status}
                results={runState.results}
                onCancel={() => void cancel()}
                onRunAgain={() => void run()}
                cancelling={cancelling}
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
