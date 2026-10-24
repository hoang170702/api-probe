import { Button, Flex, Select, Table, Tag, Typography } from 'antd'
import type { Field, FieldRule, FieldType } from '../types/api'

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
  const required = fields.filter((f) => rules[f.path] === 'REQUIRED').length

  return (
    <Table<Field>
      size="small"
      rowKey="path"
      pagination={false}
      dataSource={fields}
      scroll={{ x: 'max-content' }}
      locale={{ emptyText: 'The body has no fields' }}
      title={() => (
        <Flex justify="space-between" align="center" wrap gap={8}>
          <Typography.Text strong>
            Fields ({fields.length}) · {required} Required · {fields.length - required} Unrequired
          </Typography.Text>
          <Flex gap={8}>
            <Button size="small" onClick={() => onSetAll('REQUIRED')}>
              All Required
            </Button>
            <Button size="small" onClick={() => onSetAll('UNREQUIRED')}>
              All Unrequired
            </Button>
          </Flex>
        </Flex>
      )}
      columns={[
        { title: '#', width: 56, render: (_, __, i) => i + 1 },
        {
          title: 'Field path',
          dataIndex: 'path',
          render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
        },
        {
          title: 'Type',
          dataIndex: 'type',
          width: 110,
          render: (t: FieldType) => <Tag color={TYPE_COLORS[t]}>{t}</Tag>,
        },
        {
          title: 'Sample value',
          dataIndex: 'sampleValue',
          render: (v: string) => (
            <Typography.Text
              ellipsis={{ tooltip: v }}
              style={{ maxWidth: 360, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' }}
            >
              {v}
            </Typography.Text>
          ),
        },
        {
          title: 'Rule',
          width: 150,
          render: (_, f) => (
            <Select<FieldRule>
              size="small"
              style={{ width: 130 }}
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
