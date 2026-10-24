import { Button, Flex, Select, Table, Tag, Typography } from 'antd'
import type { Field, FieldRule, FieldType } from '../types/api'
import { useI18n } from '../i18n/context'

const TYPE_COLORS: Record<FieldType, string> = {
  string: 'green',
  number: 'blue',
  boolean: 'purple',
  null: 'default',
  object: 'cyan',
  array: 'geekblue',
}

const RULE_OPTIONS = [
  { value: 'REQUIRED', label: 'Required' },
  { value: 'UNREQUIRED', label: 'Unrequired' },
]

interface Props {
  fields: Field[]
  rules: Record<string, FieldRule>
  onRuleChange: (path: string, rule: FieldRule) => void
  onSetAll: (rule: FieldRule) => void
}

/** Every field of the JSON body with its Required/Unrequired rule (BRD-001 §7, D3). */
export default function FieldRuleTable({ fields, rules, onRuleChange, onSetAll }: Props) {
  const { t } = useI18n()
  const required = fields.filter((f) => rules[f.path] === 'REQUIRED').length

  return (
    <Table<Field>
      size="small"
      rowKey="path"
      pagination={false}
      dataSource={fields}
      scroll={{ x: 'max-content' }}
      locale={{ emptyText: t('fields.empty') }}
      title={() => (
        <Flex justify="space-between" align="center" wrap gap={8}>
          <Typography.Text strong>
            {t('fields.summary', { n: fields.length, required, unrequired: fields.length - required })}
          </Typography.Text>
          <Flex gap={8}>
            <Button size="small" onClick={() => onSetAll('REQUIRED')}>
              {t('fields.allRequired')}
            </Button>
            <Button size="small" onClick={() => onSetAll('UNREQUIRED')}>
              {t('fields.allUnrequired')}
            </Button>
          </Flex>
        </Flex>
      )}
      columns={[
        {
          title: t('fields.path'),
          dataIndex: 'path',
          render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
        },
        {
          title: t('fields.type'),
          dataIndex: 'type',
          width: 90,
          render: (t: FieldType) => <Tag color={TYPE_COLORS[t]}>{t}</Tag>,
        },
        {
          title: t('fields.sample'),
          dataIndex: 'sampleValue',
          render: (v: string) => (
            <Typography.Text
              ellipsis={{ tooltip: v }}
              style={{ maxWidth: 120, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' }}
            >
              {v}
            </Typography.Text>
          ),
        },
        {
          title: t('fields.rule'),
          width: 130,
          render: (_, f) => (
            <Select<FieldRule>
              size="small"
              style={{ width: 118 }}
              value={rules[f.path] ?? 'REQUIRED'}
              options={RULE_OPTIONS}
              onChange={(rule) => onRuleChange(f.path, rule)}
            />
          ),
        },
      ]}
    />
  )
}
