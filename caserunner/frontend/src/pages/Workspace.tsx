import { useEffect, useRef, useState } from 'react'
import { cancelRun, startRun, watchRun } from '../api/runs'
import { Alert, App, Button, Card, Col, Collapse, ConfigProvider, Empty, Flex, Grid, Layout, Row, Segmented, Tag, theme as antdTheme, Tooltip, Typography, type CollapseProps } from 'antd'
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
import { countStatuses, FINISHED, remapRows, rowProblems, sampleRow, STATUS_COLOR } from '../lib/cases'
import { downloadConfig, parseConfig, type ConfigFile } from '../lib/config-file'

type Health = { state: 'checking' } | { state: 'ok'; version: string } | { state: 'down'; message: string }

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/**
 * The single API Case Runner page (BRD-001 §16): a top bar with Run and the
 * live summary; configuration (token steps, curl, request, field rules) on
 * the left; the case table and the results on the right. Narrow screens
 * stack the two columns.
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
  const [openSections, setOpenSections] = useState<string[]>(['curl'])
  const screens = Grid.useBreakpoint()
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
    // Open the next thing to check: the field rules, or the request when the body cannot run.
    const section = res.template.bodyKind === 'json' ? 'fields' : 'request'
    setOpenSections((prev) => (prev.includes(section) ? prev : [...prev, section]))
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
      if (restored.length > 0) setOpenSections((prev) => (prev.includes('token') ? prev : [...prev, 'token']))
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

  const isJson = parsed?.template.bodyKind === 'json'
  const running = runState?.status === 'RUNNING'
  const counts = runState ? countStatuses(runState.results) : undefined
  const required = parsed ? parsed.fields.filter((f) => (rules[f.path] ?? 'REQUIRED') === 'REQUIRED').length : 0

  const sections: CollapseProps['items'] = [
    {
      key: 'token',
      label: <SectionLabel title={t('prepare.card')} summary={steps.length > 0 ? t('summary.tokenSteps', { n: steps.length }) : t('summary.tokenNone')} />,
      children: (
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
      ),
    },
    {
      key: 'curl',
      label: <SectionLabel title={t('curl.card')} summary={parsed ? `${parsed.template.method} ${parsed.template.url}` : t('summary.curlEmpty')} />,
      children: <CurlInput value={curl} onChange={setCurl} onAnalyze={analyze} loading={analyzing} error={parseError} />,
    },
    ...(parsed
      ? [
          {
            key: 'request',
            label: (
              <SectionLabel
                title={t('request.card')}
                summary={[
                  t(`body.${parsed.template.bodyKind}`),
                  `${parsed.template.headers.length} header`,
                  ...(parsed.warnings.length > 0 ? [t('request.warnings', { n: parsed.warnings.length })] : []),
                ].join(' · ')}
              />
            ),
            children: <RequestSummary template={parsed.template} warnings={parsed.warnings} injectToken={tokenInjected} knownVars={knownVars} />,
          },
        ]
      : []),
    ...(parsed && isJson
      ? [
          {
            key: 'fields',
            label: <SectionLabel title={t('fields.card')} summary={t('summary.fields', { n: parsed.fields.length, required })} />,
            children: (
              <FieldRuleTable
                fields={parsed.fields}
                rules={rules}
                onRuleChange={(path, rule) => setRules((prev) => ({ ...prev, [path]: rule }))}
                onSetAll={(rule) => setRules(Object.fromEntries(parsed.fields.map((f) => [f.path, rule])))}
              />
            ),
          },
        ]
      : []),
  ]

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Header style={{ position: 'sticky', top: 0, zIndex: 10, display: 'flex', alignItems: 'center', gap: 16 }}>
        <Flex align="center" gap={10} style={{ minWidth: 0 }}>
          <span
            style={{ width: 28, height: 28, lineHeight: '28px', textAlign: 'center', borderRadius: 7, background: '#4f46e5', color: '#fff', fontWeight: 700, fontSize: 12 }}
          >
            AC
          </span>
          <Typography.Text strong style={{ color: '#fff', fontSize: 16, whiteSpace: 'nowrap' }}>
            {t('app.title')}
          </Typography.Text>
          {screens.lg && <Typography.Text style={{ color: '#9ca3af', whiteSpace: 'nowrap' }}>{t('app.tagline')}</Typography.Text>}
        </Flex>
        <ConfigProvider theme={{ algorithm: antdTheme.darkAlgorithm, token: { colorPrimary: '#6366f1' } }}>
          <Flex align="center" gap={8} style={{ marginInlineStart: 'auto' }}>
            {counts && screens.lg && (
              <Flex gap={4} wrap>
                {FINISHED.filter((s) => counts[s] > 0).map((s) => (
                  <Tag key={s} color={STATUS_COLOR[s]} style={{ marginInlineEnd: 0 }}>
                    {s} {counts[s]}
                  </Tag>
                ))}
              </Flex>
            )}
            {running ? (
              <Button danger type="primary" onClick={() => void cancel()} loading={cancelling}>
                {t('results.cancel')}
              </Button>
            ) : (
              <Tooltip title={parsed && !stepsReady ? t('run.notReady') : undefined}>
                <Button type="primary" onClick={() => void run()} loading={starting} disabled={!isJson || rows.length === 0 || !stepsReady}>
                  {runState ? t('results.runAgain') : t('run.button')}
                </Button>
              </Tooltip>
            )}
            <HealthTag health={health} />
          </Flex>
        </ConfigProvider>
        <Segmented<Lang>
          size="small"
          aria-label={t('lang.switch')}
          value={lang}
          onChange={setLang}
          options={[
            { label: 'VI', value: 'vi' },
            { label: 'EN', value: 'en' },
          ]}
        />
      </Layout.Header>

      <Layout.Content style={{ padding: 20, maxWidth: 1760, width: '100%', margin: '0 auto' }}>
        {health.state === 'down' && <Alert style={{ marginBottom: 16 }} type="error" showIcon title={t('health.unreachable')} description={health.message} />}
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
        <Row gutter={[20, 20]} align="top">
          <Col xs={24} xl={10} xxl={8} style={screens.xl ? { position: 'sticky', top: 76 } : undefined}>
            <Card
              title={t('layout.config')}
              styles={{ body: { padding: 0, ...(screens.xl && { maxHeight: 'calc(100vh - 150px)', overflow: 'auto' }) } }}
              extra={
                <Flex gap={8}>
                  <Button size="small" onClick={() => fileInput.current?.click()}>
                    {t('config.import')}
                  </Button>
                  <Button size="small" disabled={!isJson} onClick={() => downloadConfig(currentConfig())}>
                    {t('config.export')}
                  </Button>
                </Flex>
              }
            >
              <Collapse bordered={false} activeKey={openSections} onChange={(keys) => setOpenSections(keys as string[])} items={sections} />
            </Card>
          </Col>

          <Col xs={24} xl={14} xxl={16}>
            <Flex vertical gap={20}>
              {!parsed && (
                <Card>
                  <Empty description={t('layout.empty')} />
                </Card>
              )}
              {parsed && !isJson && <Alert type="warning" showIcon title={t('request.unsupported')} description={t('request.unsupportedDesc')} />}
              {parsed && isJson && (
                <Card title={t('cases.card')}>
                  <Flex vertical gap={16}>
                    <CaseTable fields={parsed.fields} rules={rules} rows={rows} onChange={setRows} onAdd={addRow} onDuplicate={duplicateRow} />
                    <RunOptions value={options} onChange={setOptions} sequential={steps.length > 0} />
                  </Flex>
                </Card>
              )}
              {parsed && isJson && (
                <Card title={t('results.card')}>
                  {runState ? (
                    <ResultView status={runState.status} results={runState.results} />
                  ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('results.empty')} />
                  )}
                </Card>
              )}
            </Flex>
          </Col>
        </Row>
      </Layout.Content>
    </Layout>
  )
}

// SectionLabel is a collapsible section's title with a one-line summary, readable while it is closed.
function SectionLabel({ title, summary }: { title: string; summary: string }) {
  return (
    <Flex justify="space-between" align="center" gap={12} style={{ minWidth: 0 }}>
      <Typography.Text strong style={{ whiteSpace: 'nowrap' }}>
        {title}
      </Typography.Text>
      <Typography.Text type="secondary" ellipsis style={{ fontSize: 12, maxWidth: '65%' }}>
        {summary}
      </Typography.Text>
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
