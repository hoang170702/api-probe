import { Flex, InputNumber, Typography } from 'antd'
import type { RunOptionsValue } from '../types/api'
import { useI18n } from '../i18n/context'

interface Props {
  value: RunOptionsValue
  onChange: (value: RunOptionsValue) => void
  /** Token steps make cases run one at a time (BRD-001 D21). */
  sequential: boolean
}

/** Concurrency and per-request timeout of a run (BRD-001 §16, OD4). */
export default function RunOptions({ value, onChange, sequential }: Props) {
  const { t } = useI18n()
  return (
    <Flex gap={24} align="center" wrap>
      <Flex gap={8} align="center">
        <Typography.Text>{t('options.concurrency')}</Typography.Text>
        <InputNumber<number>
          min={1}
          max={20}
          value={sequential ? 1 : value.concurrency}
          disabled={sequential}
          onChange={(v) => onChange({ ...value, concurrency: v ?? 5 })}
        />
      </Flex>
      <Flex gap={8} align="center">
        <Typography.Text>{t('options.timeout')}</Typography.Text>
        <InputNumber<number> min={1} max={120} value={value.timeoutSec} onChange={(v) => onChange({ ...value, timeoutSec: v ?? 30 })} />
      </Flex>
      {sequential && <Typography.Text type="secondary">{t('options.sequential')}</Typography.Text>}
    </Flex>
  )
}
