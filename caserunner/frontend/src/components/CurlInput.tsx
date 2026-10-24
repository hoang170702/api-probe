import { Alert, Button, Flex, Input, Typography } from 'antd'
import { useI18n } from '../i18n/context'

const PLACEHOLDER = `curl 'https://uat-api.example.com/v1/transfers' \\
  -H 'Content-Type: application/json' \\
  --data-raw '{"requestId":"REQ001","data":{"amount":100000}}'`

interface Props {
  value: string
  onChange: (value: string) => void
  onAnalyze: () => void
  loading: boolean
  error?: string
}

/** Curl panel (BRD-001 §16): paste a command and analyze it. Ctrl/⌘+Enter also analyzes. */
export default function CurlInput({ value, onChange, onAnalyze, loading, error }: Props) {
  const { t } = useI18n()
  return (
    <Flex vertical gap={12}>
      <Typography.Text type="secondary">{t('curl.help')}</Typography.Text>
      <Input.TextArea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && value.trim() !== '') onAnalyze()
        }}
        placeholder={PLACEHOLDER}
        autoSize={{ minRows: 6, maxRows: 18 }}
        spellCheck={false}
        style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace', fontSize: 13 }}
      />
      <Flex gap={12} align="center">
        <Button type="primary" onClick={onAnalyze} loading={loading} disabled={value.trim() === ''}>
          {t('curl.analyze')}
        </Button>
        <Typography.Text type="secondary">{t('curl.shortcut')}</Typography.Text>
      </Flex>
      {error && <Alert type="error" showIcon title={t('curl.error')} description={error} />}
    </Flex>
  )
}
