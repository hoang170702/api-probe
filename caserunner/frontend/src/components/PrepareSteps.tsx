import { useState } from 'react'
import { Alert, Button, Card, Collapse, Flex, Input, Space, Switch, Tag, Typography } from 'antd'
import type { Capture, ParseResponse, StepResult } from '../types/api'
import { useI18n } from '../i18n/context'
import { maskValue } from '../lib/mask'

const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'

export interface PrepareStepState {
  id: number
  curl: string
  parsed?: ParseResponse
  parseError?: string
  analyzing: boolean
  captures: Capture[]
}

interface Props {
  steps: PrepareStepState[]
  onAdd: () => void
  onRemove: (id: number) => void
  onCurlChange: (id: number, curl: string) => void
  onAnalyze: (id: number) => void
  onCapturesChange: (id: number, captures: Capture[]) => void
  injectToken: boolean
  onInjectTokenChange: (on: boolean) => void
  onTry: () => void
  trying: boolean
  tryError?: string
  results?: StepResult[]
}

const STATUS_COLOR: Record<StepResult['status'], string> = { OK: 'success', FAILED: 'error', SKIPPED: 'default' }

/** Optional token steps run before every case (BRD-001 §11.3). */
export default function PrepareSteps(props: Props) {
  const { t } = useI18n()
  const { steps, results } = props
  const [showSecrets, setShowSecrets] = useState(false)
  const allParsed = steps.length > 0 && steps.every((s) => s.parsed)

  return (
    <Flex vertical gap={16}>
      <Typography.Text type="secondary">{t('prepare.intro')}</Typography.Text>

      {steps.map((step, i) => (
        <Card
          key={step.id}
          size="small"
          title={t('prepare.step', { n: i + 1 })}
          extra={
            <Button size="small" danger type="text" onClick={() => props.onRemove(step.id)}>
              {t('prepare.remove')}
            </Button>
          }
        >
          <Flex vertical gap={12}>
            <Input.TextArea
              value={step.curl}
              onChange={(e) => props.onCurlChange(step.id, e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && step.curl.trim() !== '') props.onAnalyze(step.id)
              }}
              placeholder={t('prepare.curlPlaceholder')}
              autoSize={{ minRows: 3, maxRows: 12 }}
              spellCheck={false}
              style={{ fontFamily: MONO, fontSize: 13 }}
            />
            <Flex gap={12} align="center" wrap>
              <Button onClick={() => props.onAnalyze(step.id)} loading={step.analyzing} disabled={step.curl.trim() === ''}>
                {t('curl.analyze')}
              </Button>
              {step.parsed && (
                <Flex gap={8} align="center">
                  <Tag color="blue">{step.parsed.template.method}</Tag>
                  <Typography.Text code style={{ wordBreak: 'break-all' }}>
                    {step.parsed.template.url}
                  </Typography.Text>
                </Flex>
              )}
            </Flex>
            {step.parseError && <Alert type="error" showIcon title={t('curl.error')} description={step.parseError} />}

            <CaptureEditor captures={step.captures} onChange={(c) => props.onCapturesChange(step.id, c)} />

            {results?.[i] && <StepOutcome result={results[i]} showSecrets={showSecrets} />}
          </Flex>
        </Card>
      ))}

      <Flex gap={12} align="center" wrap>
        <Button onClick={props.onAdd}>+ {t('prepare.add')}</Button>
        {steps.length > 0 && (
          <>
            <Button type="primary" onClick={props.onTry} loading={props.trying} disabled={!allParsed}>
              {t('prepare.try')}
            </Button>
            {!allParsed && <Typography.Text type="secondary">{t('prepare.needAnalyze')}</Typography.Text>}
            <Flex gap={8} align="center" style={{ marginInlineStart: 'auto' }}>
              <Typography.Text type="secondary">{t('headers.showSensitive')}</Typography.Text>
              <Switch size="small" checked={showSecrets} onChange={setShowSecrets} />
            </Flex>
          </>
        )}
      </Flex>

      {steps.length > 0 && (
        <Flex vertical gap={4}>
          <Flex gap={8} align="center">
            <Switch checked={props.injectToken} onChange={props.onInjectTokenChange} />
            <Typography.Text>{t('prepare.inject')}</Typography.Text>
          </Flex>
          <Typography.Text type="secondary">{t('prepare.useVars')}</Typography.Text>
        </Flex>
      )}
      {props.tryError && <Alert type="error" showIcon title={t('prepare.tryFailed')} description={props.tryError} />}
    </Flex>
  )
}

function CaptureEditor({ captures, onChange }: { captures: Capture[]; onChange: (c: Capture[]) => void }) {
  const { t } = useI18n()
  const update = (i: number, patch: Partial<Capture>) => onChange(captures.map((c, j) => (j === i ? { ...c, ...patch } : c)))

  return (
    <Flex vertical gap={8}>
      <Typography.Text strong>{t('prepare.captures')}</Typography.Text>
      {captures.map((c, i) => (
        <Flex key={i} gap={8} align="center" wrap>
          <Space.Compact style={{ width: 260 }}>
            <Space.Addon>{t('prepare.var')}</Space.Addon>
            <Input
              value={c.var}
              onChange={(e) => update(i, { var: e.target.value })}
              style={{ fontFamily: MONO }}
              status={/^[A-Za-z_][A-Za-z0-9_]*$/.test(c.var) ? undefined : 'error'}
            />
          </Space.Compact>
          <Space.Compact style={{ flex: 1, minWidth: 280 }}>
            <Space.Addon>{t('prepare.path')}</Space.Addon>
            <Input
              value={c.path}
              placeholder={t('prepare.pathAuto')}
              onChange={(e) => update(i, { path: e.target.value })}
              style={{ fontFamily: MONO }}
            />
          </Space.Compact>
          <Button size="small" type="text" onClick={() => onChange(captures.filter((_, j) => j !== i))} disabled={captures.length === 1}>
            {t('prepare.removeCapture')}
          </Button>
        </Flex>
      ))}
      <PathHelp />
      <div>
        <Button size="small" type="dashed" onClick={() => onChange([...captures, { var: '', path: '' }])}>
          + {t('prepare.addCapture')}
        </Button>
      </div>
    </Flex>
  )
}

// Explains the path syntax with a concrete response, so testers know what to type.
function PathHelp() {
  const { t } = useI18n()
  const code = (s: string) => (
    <Typography.Text code style={{ fontSize: 12 }}>
      {s}
    </Typography.Text>
  )
  // The example sentence has three slots, filled with code-formatted values.
  const parts = t('prepare.pathExample').split(/(\{json\}|\{path\}|\{arrayPath\})/)
  const slots: Record<string, string> = {
    '{json}': '{"data":{"accessToken":"eyJ…","items":[{"id":"A1"}]}}',
    '{path}': 'data.accessToken',
    '{arrayPath}': 'data.items[0].id',
  }
  return (
    <Typography.Text type="secondary" style={{ fontSize: 13 }}>
      {t('prepare.pathHelp')}{' '}
      {parts.map((p, i) => (slots[p] ? <span key={i}>{code(slots[p])}</span> : <span key={i}>{p}</span>))}
    </Typography.Text>
  )
}

function StepOutcome({ result, showSecrets }: { result: StepResult; showSecrets: boolean }) {
  const { t } = useI18n()
  return (
    <Flex vertical gap={8}>
      <Flex gap={8} align="center" wrap>
        <Tag color={STATUS_COLOR[result.status]}>{t(`prepare.status.${result.status}`)}</Tag>
        {result.httpStatus !== undefined && (
          <Typography.Text type="secondary">{t('prepare.httpTime', { code: result.httpStatus, ms: result.durationMs })}</Typography.Text>
        )}
        {result.reason && <Typography.Text type={result.status === 'FAILED' ? 'danger' : 'secondary'}>{result.reason}</Typography.Text>}
      </Flex>
      {result.captured.map((c) => (
        <Flex key={c.var} gap={8} align="center" wrap>
          <Typography.Text>{t('prepare.captured')}</Typography.Text>
          <Typography.Text code>{`{{${c.var}}}`}</Typography.Text>
          <Typography.Text style={{ fontFamily: MONO, wordBreak: 'break-all' }}>{showSecrets ? c.value : maskValue(c.value)}</Typography.Text>
          <Typography.Text type="secondary">{t('prepare.foundAt', { path: c.path })}</Typography.Text>
        </Flex>
      ))}
      {result.responseBody !== '' && (
        <Collapse
          size="small"
          items={[
            {
              key: 'body',
              label: `${t('prepare.response')} ${result.truncated ? t('prepare.truncated') : ''}`,
              children: (
                <pre style={{ margin: 0, maxHeight: 320, overflow: 'auto', fontFamily: MONO, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                  {showSecrets ? prettyJSON(result.responseBody) : maskCaptured(prettyJSON(result.responseBody), result)}
                </pre>
              ),
            },
          ]}
        />
      )}
    </Flex>
  )
}

// Captured values are secrets (§11.3): hide them in the body as well.
function maskCaptured(text: string, result: StepResult): string {
  return result.captured.reduce((out, c) => (c.value ? out.replaceAll(c.value, maskValue(c.value)) : out), text)
}

function prettyJSON(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}
