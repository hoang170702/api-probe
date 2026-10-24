import { useState } from 'react'
import { Alert, Descriptions, Flex, Switch, Table, Tag, Typography } from 'antd'
import type { Header, RequestTemplate } from '../types/api'
import { useI18n } from '../i18n/context'
import { isSensitiveHeader, maskValue } from '../lib/mask'
import { placeholders } from '../lib/placeholders'

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
  /** Authorization will be replaced by Bearer {{token}} at run time (§11.3). */
  injectToken: boolean
  /** Variables the token steps capture. */
  knownVars: string[]
}

/** The request as parsed, for the tester to confirm before writing cases (BRD-001 §16). */
export default function RequestSummary({ template, warnings, injectToken, knownVars }: Props) {
  const { t } = useI18n()
  const [showSecrets, setShowSecrets] = useState(false)
  const rows = template.headers.map((h, i) => ({ ...h, key: i }))
  const used = placeholders(template)

  return (
    <Flex vertical gap={16}>
      <Descriptions
        size="small"
        column={1}
        items={[
          {
            label: t('request.request'),
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
          { label: t('request.body'), children: <BodyKindTag kind={template.bodyKind} /> },
          ...(used.length > 0
            ? [
                {
                  label: t('vars.title'),
                  children: (
                    <Flex gap={8} wrap>
                      {used.map((name) =>
                        knownVars.includes(name) ? (
                          <Tag key={name} color="success">{`{{${name}}}`}</Tag>
                        ) : (
                          <Tag key={name} color="error">{`{{${name}}} · ${t('vars.missing')}`}</Tag>
                        ),
                      )}
                    </Flex>
                  ),
                },
              ]
            : []),
        ]}
      />

      {template.bodyKind === 'unsupported' && (
        <Alert type="error" showIcon title={t('request.unsupported')} description={t('request.unsupportedDesc')} />
      )}
      {warnings.length > 0 && (
        <Alert
          type="warning"
          showIcon
          title={t('request.warnings', { n: warnings.length })}
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
        locale={{ emptyText: t('headers.none') }}
        title={() => (
          <Flex justify="space-between" align="center">
            <Typography.Text strong>{t('headers.title', { n: rows.length })}</Typography.Text>
            <Flex gap={8} align="center">
              <Typography.Text type="secondary">{t('headers.showSensitive')}</Typography.Text>
              <Switch size="small" checked={showSecrets} onChange={setShowSecrets} />
            </Flex>
          </Flex>
        )}
        columns={[
          {
            title: t('headers.name'),
            dataIndex: 'name',
            width: 260,
            render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
          },
          {
            title: t('headers.value'),
            dataIndex: 'value',
            render: (v: string, h) => {
              const replaced = injectToken && h.name.toLowerCase() === 'authorization'
              return (
                <Typography.Text delete={replaced} style={{ wordBreak: 'break-all' }}>
                  {isSensitiveHeader(h.name) && !showSecrets ? maskValue(v) : v}
                  {replaced && (
                    <Tag color="processing" style={{ marginInlineStart: 8 }}>
                      {t('headers.replaced')}
                    </Tag>
                  )}
                </Typography.Text>
              )
            },
          },
        ]}
      />
    </Flex>
  )
}

function BodyKindTag({ kind }: { kind: RequestTemplate['bodyKind'] }) {
  const { t } = useI18n()
  switch (kind) {
    case 'json':
      return <Tag color="blue">{t('body.json')}</Tag>
    case 'none':
      return <Tag>{t('body.none')}</Tag>
    default:
      return <Tag color="error">{t('body.unsupported')}</Tag>
  }
}
