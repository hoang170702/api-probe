// Header masking of BRD-001 §13: sensitive values show only their first
// 6 characters followed by ***.

const SENSITIVE_NAMES = new Set(['authorization', 'proxy-authorization', 'cookie', 'set-cookie', 'x-api-key'])

export function isSensitiveHeader(name: string): boolean {
  const n = name.toLowerCase()
  return SENSITIVE_NAMES.has(n) || n.includes('token') || n.includes('secret')
}

export function maskValue(value: string): string {
  return value.slice(0, 6) + '***'
}
