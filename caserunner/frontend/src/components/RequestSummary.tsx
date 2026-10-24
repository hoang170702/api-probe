import { useState } from 'react'
import { Alert, Descriptions, Flex, Switch, Table, Tag, Typography } from 'antd'
import type { Header, RequestTemplate } from '../types/api'
import { isSensitiveHeader, maskValue } from '../lib/mask'

const METHOD_COLORS: Record<string, string> = {
  GET: 'green',
  POST: 'blue',
  PUT: 'orange',
  PATCH: 'gold',
  DELETE: 'red',
}

interface Props {
  template: RequestTemplate
  warnings: string[]
}

/** The request as parsed, for the tester to confirm before writing cases (BRD-001 §16). */
export default function RequestSummary({ template, warnings }: Props) {
  const [showSecrets, setShowSecrets] = useState(false)
  const rows = template.headers.map((h, i) => ({ ...h, key: i }))

  return (
    <Flex vertical gap={16}>
      <Descriptions
        size="small"
        column={1}
        items={[
          {
            label: 'Request',
            children: (
              <Flex gap={8} align="center" wrap>
                <Tag color={METHOD_COLORS[template.method.toUpperCase()] ?? 'default'}>{template.method}</Tag>
                <Typography.Text code copyable style={{ wordBreak: 'break-all' }}>
                  {template.url}
                </Typography.Text>
                {template.insecure && <Tag color="warning">-k / --insecure</Tag>}
              </Flex>
            ),
          },
          { label: 'Body', children: <BodyKindTag kind={template.bodyKind} /> },
        ]}
      />

      {template.bodyKind === 'unsupported' && (
        <Alert type="error" showIcon title="v1 supports JSON bodies only" description="This request cannot be run." />
      )}
      {warnings.length > 0 && (
        <Alert
          type="warning"
          showIcon
          title={`${warnings.length} warning${warnings.length > 1 ? 's' : ''}`}
          description={
            <ul style={{ margin: 0, paddingInlineStart: 20 }}>
              {warnings.map((w, i) => (
                <li key={i}>{w}</li>
              ))}
            </ul>
          }
        />
      )}

      <Table<Header & { key: number }>
        size="small"
        pagination={false}
        dataSource={rows}
        locale={{ emptyText: 'No headers' }}
        title={() => (
          <Flex justify="space-between" align="center">
            <Typography.Text strong>Headers ({rows.length})</Typography.Text>
            <Flex gap={8} align="center">
              <Typography.Text type="secondary">Show sensitive values</Typography.Text>
              <Switch size="small" checked={showSecrets} onChange={setShowSecrets} />
            </Flex>
          </Flex>
        )}
        columns={[
          { title: 'Name', dataIndex: 'name', width: 260, render: (v: string) => <Typography.Text code>{v}</Typography.Text> },
          {
            title: 'Value',
            dataIndex: 'value',
            render: (v: string, h) => (
              <Typography.Text style={{ wordBreak: 'break-all' }}>
                {isSensitiveHeader(h.name) && !showSecrets ? maskValue(v) : v}
              </Typography.Text>
            ),
          },
        ]}
      />
    </Flex>
  )
}

function BodyKindTag({ kind }: { kind: RequestTemplate['bodyKind'] }) {
  switch (kind) {
    case 'json':
      return <Tag color="blue">JSON</Tag>
    case 'none':
      return <Tag>No body</Tag>
    default:
      return <Tag color="error">Unsupported</Tag>
  }
}
