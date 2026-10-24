import { useMemo, useState, type ReactNode } from 'react'
import { Alert, Descriptions, Drawer, Flex, Progress, Segmented, Table, Tag, Typography } from 'antd'
import type { CaseResult, CaseStatus, RunStatus } from '../types/api'
import { useI18n } from '../i18n/context'
import { countStatuses, FINISHED, STATUS_COLOR } from '../lib/cases'

const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'

interface Props {
  status: RunStatus
  results: CaseResult[]
}

/** The results panel (BRD-001 §16): progress, filter, table and a detail drawer. Run and Cancel live in the top bar. */
export default function ResultView({ status, results }: Props) {
  const { t } = useI18n()
  const [filter, setFilter] = useState<'ALL' | CaseStatus>('ALL')
  const [open, setOpen] = useState<CaseResult>()

  const counts = useMemo(() => countStatuses(results), [results])
  const finished = FINISHED.reduce((n, s) => n + counts[s], 0)
  const shown = filter === 'ALL' ? results : results.filter((r) => r.status === filter)

  return (
    <Flex vertical gap={16}>
      <Progress
        percent={results.length ? Math.round((finished / results.length) * 100) : 0}
        format={() => `${finished}/${results.length}`}
        status={status === 'RUNNING' ? 'active' : status === 'CANCELLED' ? 'exception' : 'success'}
      />
      <Flex justify="space-between" align="center" wrap gap={8}>
        <Segmented<'ALL' | CaseStatus>
          value={filter}
          onChange={setFilter}
          options={[
            { label: `${t('results.all')} (${results.length})`, value: 'ALL' },
            ...FINISHED.map((s) => ({ label: `${s} (${counts[s]})`, value: s })),
          ]}
        />
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {t('results.ttl')}
        </Typography.Text>
      </Flex>
      <Table<CaseResult>
        size="small"
        rowKey="caseIndex"
        dataSource={shown}
        pagination={shown.length > 50 ? { pageSize: 50, showSizeChanger: false } : false}
        onRow={(r) => ({ onClick: () => setOpen(r), style: { cursor: 'pointer' } })}
        scroll={{ x: 'max-content' }}
        columns={[
          { title: '#', dataIndex: 'caseNo', width: 56, render: (n: number) => `${n}.` },
          { title: t('results.name'), dataIndex: 'name', width: 140 },
          {
            title: t('cases.expect'),
            dataIndex: 'expect',
            width: 160,
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
              <Typography.Text ellipsis={{ tooltip: v }} style={{ maxWidth: 360 }}>
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
