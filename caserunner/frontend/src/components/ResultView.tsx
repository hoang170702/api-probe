import { useMemo, useState, type ReactNode } from 'react'
import { Alert, Button, Descriptions, Drawer, Flex, Progress, Segmented, Table, Tag, Typography } from 'antd'
import type { CaseResult, CaseStatus, RunStatus } from '../types/api'
import { useI18n } from '../i18n/context'

const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'

const STATUS_COLOR: Record<CaseStatus, string> = {
  PENDING: 'default',
  RUNNING: 'processing',
  SUCCESS: 'success',
  FAILED: 'error',
  EXCEPTION: 'orange',
  BLOCKED: 'default',
  CANCELLED: 'default',
}

const FINISHED: CaseStatus[] = ['SUCCESS', 'FAILED', 'EXCEPTION', 'BLOCKED', 'CANCELLED']

interface Props {
  status: RunStatus
  results: CaseResult[]
  onCancel: () => void
  onRunAgain: () => void
  cancelling: boolean
}

/** The results panel (BRD-001 §16): live summary, table, filter and a detail drawer. */
export default function ResultView({ status, results, onCancel, onRunAgain, cancelling }: Props) {
  const { t } = useI18n()
  const [filter, setFilter] = useState<'ALL' | CaseStatus>('ALL')
  const [open, setOpen] = useState<CaseResult>()

  const counts = useMemo(() => {
    const c: Record<CaseStatus, number> = { PENDING: 0, RUNNING: 0, SUCCESS: 0, FAILED: 0, EXCEPTION: 0, BLOCKED: 0, CANCELLED: 0 }
    for (const r of results) c[r.status]++
    return c
  }, [results])
  const finished = FINISHED.reduce((n, s) => n + counts[s], 0)
  const shown = filter === 'ALL' ? results : results.filter((r) => r.status === filter)

  return (
    <Flex vertical gap={16}>
      <Flex gap={8} align="center" wrap>
        <Tag>{t('results.total', { n: results.length })}</Tag>
        {(['SUCCESS', 'FAILED', 'EXCEPTION', 'BLOCKED', 'CANCELLED'] as CaseStatus[]).map((s) => (
          <Tag key={s} color={STATUS_COLOR[s]}>
            {s} {counts[s]}
          </Tag>
        ))}
        <Flex gap={8} style={{ marginInlineStart: 'auto' }}>
          {status === 'RUNNING' ? (
            <Button danger onClick={onCancel} loading={cancelling}>
              {t('results.cancel')}
            </Button>
          ) : (
            <Button onClick={onRunAgain}>{t('results.runAgain')}</Button>
          )}
        </Flex>
      </Flex>
      <Progress
        percent={results.length ? Math.round((finished / results.length) * 100) : 0}
        status={status === 'RUNNING' ? 'active' : status === 'CANCELLED' ? 'exception' : 'success'}
      />
      <Alert type="info" showIcon title={t('results.ttl')} />

      <Segmented<'ALL' | CaseStatus>
        value={filter}
        onChange={setFilter}
        options={[
          { label: t('results.all'), value: 'ALL' },
          ...FINISHED.map((s) => ({ label: `${s} (${counts[s]})`, value: s })),
        ]}
      />
      <Table<CaseResult>
        size="small"
        rowKey="caseIndex"
        dataSource={shown}
        pagination={shown.length > 50 ? { pageSize: 50, showSizeChanger: false } : false}
        onRow={(r) => ({ onClick: () => setOpen(r), style: { cursor: 'pointer' } })}
        scroll={{ x: 'max-content' }}
        columns={[
          { title: '#', dataIndex: 'caseNo', width: 56, render: (n: number) => `${n}.` },
          { title: t('results.name'), dataIndex: 'name', width: 180 },
          {
            title: t('cases.expect'),
            dataIndex: 'expect',
            width: 200,
            render: (v: string) => (
              <Typography.Text code style={{ fontSize: 12 }}>
                {v}
              </Typography.Text>
            ),
          },
          { title: 'HTTP', dataIndex: 'httpStatus', width: 70 },
          {
            title: t('results.status'),
            dataIndex: 'status',
            width: 120,
            render: (s: CaseStatus) => <Tag color={STATUS_COLOR[s]}>{s}</Tag>,
          },
          {
            title: t('results.duration'),
            dataIndex: 'durationMs',
            width: 90,
            render: (ms: number, r) => (r.status === 'PENDING' || r.status === 'BLOCKED' || (r.status === 'CANCELLED' && !r.startedAt) ? '' : `${ms} ms`),
          },
          {
            title: t('results.reason'),
            dataIndex: 'reason',
            render: (v: string) => (
              <Typography.Text ellipsis={{ tooltip: v }} style={{ maxWidth: 420 }}>
                {v}
              </Typography.Text>
            ),
          },
        ]}
      />
      <CaseDrawer result={open} onClose={() => setOpen(undefined)} />
    </Flex>
  )
}

function CaseDrawer({ result, onClose }: { result?: CaseResult; onClose: () => void }) {
  const { t } = useI18n()
  return (
    <Drawer open={result !== undefined} onClose={onClose} size="large" title={result ? `${result.caseNo}. ${result.name}` : ''}>
      {result && (
        <Flex vertical gap={16}>
          <Descriptions
            size="small"
            column={1}
            bordered
            items={[
              { label: t('results.status'), children: <Tag color={STATUS_COLOR[result.status]}>{result.status}</Tag> },
              { label: t('results.reason'), children: result.reason },
              { label: t('cases.expect'), children: <Typography.Text code>{result.expect}</Typography.Text> },
              ...(result.foundIn ? [{ label: t('results.foundIn'), children: t(`results.in.${result.foundIn}`) }] : []),
              ...(result.httpStatus ? [{ label: 'HTTP', children: result.httpStatus }] : []),
              ...(result.errorType ? [{ label: t('results.errorType'), children: result.errorType }] : []),
              ...(result.durationMs ? [{ label: t('results.duration'), children: `${result.durationMs} ms` }] : []),
            ]}
          />
          {result.errors && result.errors.length > 0 && (
            <Alert
              type="warning"
              showIcon
              title={t('results.blockedBecause')}
              description={
                <ul style={{ margin: 0, paddingInlineStart: 20 }}>
                  {result.errors.map((e, i) => (
                    <li key={i}>{e.message}</li>
                  ))}
                </ul>
              }
            />
          )}
          <Section title={t('results.input')}>
            <Table
              size="small"
              pagination={false}
              rowKey="path"
              dataSource={Object.entries(result.input).map(([path, value]) => ({ path, value }))}
              columns={[
                { title: t('fields.path'), dataIndex: 'path', render: (v: string) => <Typography.Text code>{v}</Typography.Text> },
                { title: t('headers.value'), dataIndex: 'value', render: (v: string) => <span style={{ fontFamily: MONO }}>{v}</span> },
              ]}
            />
          </Section>
          {result.requestBody && (
            <Section title={t('results.requestBody')}>
              <Code text={pretty(result.requestBody)} />
            </Section>
          )}
          {result.responseBody !== undefined && result.responseBody !== '' && (
            <Section title={`${t('prepare.response')} ${result.truncated ? t('prepare.truncated') : ''}`}>
              <Code {...bodyView(result.responseBody, result.foundIn === 'body' ? result.expect : undefined)} />
            </Section>
          )}
        </Flex>
      )}
    </Drawer>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Flex vertical gap={8}>
      <Typography.Text strong>{title}</Typography.Text>
      {children}
    </Flex>
  )
}

// Code shows text, highlighting every occurrence of mark: where the expect code was found.
function Code({ text, mark }: { text: string; mark?: string }) {
  const parts = mark ? text.split(mark) : [text]
  return (
    <pre style={{ margin: 0, maxHeight: 360, overflow: 'auto', fontFamily: MONO, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all', background: 'rgba(0,0,0,0.03)', padding: 8, borderRadius: 6 }}>
      {parts.map((p, i) => (
        <span key={i}>
          {p}
          {mark && i < parts.length - 1 && <mark>{mark}</mark>}
        </span>
      ))}
    </pre>
  )
}

// bodyView indents a JSON body for reading, unless indenting would hide the
// expect code: "code":"00" is found in the raw body but not in "code": "00".
function bodyView(body: string, mark?: string): { text: string; mark?: string } {
  const p = pretty(body)
  return { text: mark && !p.includes(mark) ? body : p, mark }
}

function pretty(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}
