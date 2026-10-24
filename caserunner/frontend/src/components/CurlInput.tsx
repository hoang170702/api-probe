import { Alert, Button, Flex, Input, Typography } from 'antd'

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
  return (
    <Flex vertical gap={12}>
      <Typography.Text type="secondary">
        Paste a curl command copied from Chrome / Firefox DevTools ("Copy as cURL", bash or cmd) or Postman.
      </Typography.Text>
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
          Analyze
        </Button>
        <Typography.Text type="secondary">Ctrl+Enter</Typography.Text>
      </Flex>
      {error && <Alert type="error" showIcon title="Cannot use this curl" description={error} />}
    </Flex>
  )
}
