import { Button, Flex, Input, Table, Tag, Tooltip, Typography } from 'antd'
import type { CaseRow, Field, FieldRule } from '../types/api'
import { useI18n } from '../i18n/context'
import { MAX_CASES, rowProblems } from '../lib/cases'

const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'

interface Props {
  fields: Field[]
  rules: Record<string, FieldRule>
  rows: CaseRow[]
  onChange: (rows: CaseRow[]) => void
  onAdd: () => void
  onDuplicate: (id: number) => void
}

/** The editable case table (BRD-001 §8): one row per case, one column per field, plus expect. */
export default function CaseTable({ fields, rules, rows, onChange, onAdd, onDuplicate }: Props) {
  const { t } = useI18n()
  const update = (id: number, patch: Partial<CaseRow>) => onChange(rows.map((r) => (r.id === id ? { ...r, ...patch } : r)))
  const blocked = rows.filter((r) => rowProblems(r, fields, rules).length > 0).length

  return (
    <Flex vertical gap={12}>
      <Typography.Text type="secondary">{t('cases.help')}</Typography.Text>
      <Table<CaseRow>
        size="small"
        rowKey="id"
        dataSource={rows}
        pagination={rows.length > 50 ? { pageSize: 50, showSizeChanger: false } : false}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: t('cases.empty') }}
        columns={[
          { title: '#', width: 44, fixed: 'left', render: (_, __, i) => `${i + 1}.` },
          {
            title: t('cases.name'),
            width: 140,
            fixed: 'left',
            render: (_, row, i) => (
              <Input
                size="small"
                value={row.name}
                placeholder={t('cases.namePlaceholder', { n: i + 1 })}
                onChange={(e) => update(row.id, { name: e.target.value })}
              />
            ),
          },
          ...fields.map((f) => {
            const required = (rules[f.path] ?? 'REQUIRED') === 'REQUIRED'
            return {
              key: f.path,
              width: 160,
              title: (
                <Flex vertical gap={2}>
                  <Typography.Text code style={{ fontSize: 12 }}>
                    {f.path}
                  </Typography.Text>
                  <Flex gap={4}>
                    <Tag style={{ marginInlineEnd: 0, fontSize: 11 }}>{f.type}</Tag>
                    <Tag color={required ? 'red' : 'default'} style={{ marginInlineEnd: 0, fontSize: 11 }}>
                      {required ? 'Required' : 'Unrequired'}
                    </Tag>
                  </Flex>
                </Flex>
              ),
              render: (_: unknown, row: CaseRow) => {
                const value = row.values[f.path] ?? ''
                const missing = required && value === ''
                return (
                  <Tooltip title={missing ? t('cases.requiredMissing', { path: f.path }) : undefined}>
                    <Input
                      size="small"
                      value={value}
                      status={missing ? 'error' : undefined}
                      style={{ fontFamily: MONO }}
                      onChange={(e) => update(row.id, { values: { ...row.values, [f.path]: e.target.value } })}
                    />
                  </Tooltip>
                )
              },
            }
          }),
          {
            title: t('cases.expect'),
            width: 180,
            fixed: 'right',
            render: (_, row) => {
              const missing = row.expect.trim() === ''
              return (
                <Tooltip title={missing ? t('cases.expectMissing') : undefined}>
                  <Input
                    size="small"
                    value={row.expect}
                    placeholder={t('cases.expectPlaceholder')}
                    status={missing ? 'error' : undefined}
                    style={{ fontFamily: MONO }}
                    onChange={(e) => update(row.id, { expect: e.target.value })}
                  />
                </Tooltip>
              )
            },
          },
          {
            title: '',
            width: 72,
            fixed: 'right',
            render: (_, row) => (
              <Flex gap={2}>
                <Tooltip title={t('cases.duplicate')}>
                  <Button size="small" type="text" aria-label={t('cases.duplicate')} disabled={rows.length >= MAX_CASES} onClick={() => onDuplicate(row.id)}>
                    <CopyIcon />
                  </Button>
                </Tooltip>
                <Tooltip title={t('cases.delete')}>
                  <Button size="small" type="text" danger aria-label={t('cases.delete')} onClick={() => onChange(rows.filter((r) => r.id !== row.id))}>
                    <DeleteIcon />
                  </Button>
                </Tooltip>
              </Flex>
            ),
          },
        ]}
      />
      <Flex gap={12} align="center" wrap>
        <Button onClick={onAdd} disabled={rows.length >= MAX_CASES}>
          + {t('cases.add')}
        </Button>
        <Typography.Text type={blocked > 0 ? 'danger' : 'secondary'}>
          {t('cases.summary', { n: rows.length, blocked })}
        </Typography.Text>
        {rows.length >= MAX_CASES && <Typography.Text type="warning">{t('cases.max', { max: MAX_CASES })}</Typography.Text>}
      </Flex>
    </Flex>
  )
}

// Small inline icons: the project has no icon package.
function CopyIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden>
      <rect x="5" y="5" width="9" height="9" rx="1.5" />
      <path d="M11 5V3.5A1.5 1.5 0 0 0 9.5 2h-6A1.5 1.5 0 0 0 2 3.5v6A1.5 1.5 0 0 0 3.5 11H5" />
    </svg>
  )
}

function DeleteIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" aria-hidden>
      <path d="M2.5 4h11M6 4V2.5h4V4M4 4l.7 9.5h6.6L12 4M6.5 7v4M9.5 7v4" />
    </svg>
  )
}
