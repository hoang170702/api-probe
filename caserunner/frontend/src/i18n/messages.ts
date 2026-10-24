// UI strings in English and Vietnamese (BRD-001 D22). Messages from the
// backend (parse errors, warnings, reasons) stay in English.

export type Lang = 'en' | 'vi'

const en = {
  'app.title': 'API Case Runner',
  'lang.switch': 'Language',
  'health.checking': 'Checking backend…',
  'health.ok': 'BE OK · {version}',
  'health.down': 'BE down',
  'health.unreachable': 'Backend unreachable',

  'prepare.card': '0. Token (optional)',
  'prepare.intro':
    'If the API needs a token, add the curl that obtains it (e.g. login). It runs before every case; the token it returns is put into the main curl.',
  'prepare.add': 'Add token step',
  'prepare.step': 'Step {n}',
  'prepare.remove': 'Remove step',
  'prepare.curlPlaceholder': "curl 'https://uat-api.example.com/auth/login' -H 'Content-Type: application/json' --data-raw '{\"username\":\"tester\",\"password\":\"…\"}'",
  'prepare.captures': 'Values to capture from the response',
  'prepare.var': 'Variable',
  'prepare.path': 'Path in the response',
  'prepare.pathAuto': 'empty = find the token automatically',
  'prepare.addCapture': 'Add variable',
  'prepare.removeCapture': 'Remove',
  'prepare.try': 'Try steps',
  'prepare.needAnalyze': 'Analyze every step before trying.',
  'prepare.inject': 'Put Authorization: Bearer {{token}} on the main curl',
  'prepare.useVars': 'Later steps and the main curl can use {{name}} anywhere in the URL, headers or body.',
  'prepare.status.OK': 'OK',
  'prepare.status.FAILED': 'Failed',
  'prepare.status.SKIPPED': 'Skipped',
  'prepare.httpTime': 'HTTP {code} · {ms} ms',
  'prepare.captured': 'Captured',
  'prepare.foundAt': 'from {path}',
  'prepare.response': 'Response body',
  'prepare.truncated': '(first 1 MB)',
  'prepare.tryFailed': 'Could not try the steps',

  'curl.card': '1. Curl',
  'curl.help': 'Paste a curl command copied from Chrome / Firefox DevTools ("Copy as cURL", bash or cmd) or Postman.',
  'curl.analyze': 'Analyze',
  'curl.shortcut': 'Ctrl+Enter',
  'curl.error': 'Cannot use this curl',

  'request.card': '2. Request',
  'request.request': 'Request',
  'request.body': 'Body',
  'body.json': 'JSON',
  'body.none': 'No body',
  'body.unsupported': 'Unsupported',
  'request.unsupported': 'v1 supports JSON bodies only',
  'request.unsupportedDesc': 'This request cannot be run.',
  'request.warnings': '{n} warning(s)',
  'headers.title': 'Headers ({n})',
  'headers.none': 'No headers',
  'headers.name': 'Name',
  'headers.value': 'Value',
  'headers.showSensitive': 'Show sensitive values',
  'headers.replaced': 'replaced by Bearer {{token}} when running',
  'vars.title': 'Variables used',
  'vars.missing': 'not captured by any token step',

  'fields.card': '3. Field rules',
  'fields.summary': 'Fields ({n}) · {required} Required · {unrequired} Unrequired',
  'fields.allRequired': 'All Required',
  'fields.allUnrequired': 'All Unrequired',
  'fields.empty': 'The body has no fields',
  'fields.path': 'Field path',
  'fields.type': 'Type',
  'fields.sample': 'Sample value',
  'fields.rule': 'Rule',
} as const

export type MessageKey = keyof typeof en

const vi: Record<MessageKey, string> = {
  'app.title': 'API Case Runner',
  'lang.switch': 'Ngôn ngữ',
  'health.checking': 'Đang kiểm tra backend…',
  'health.ok': 'BE OK · {version}',
  'health.down': 'BE lỗi',
  'health.unreachable': 'Không kết nối được backend',

  'prepare.card': '0. Lấy token (tuỳ chọn)',
  'prepare.intro':
    'Nếu API cần token, thêm curl dùng để lấy token (ví dụ login). Nó chạy trước mỗi case; token nhận được sẽ được gắn vào curl chính.',
  'prepare.add': 'Thêm bước lấy token',
  'prepare.step': 'Bước {n}',
  'prepare.remove': 'Xoá bước',
  'prepare.curlPlaceholder': "curl 'https://uat-api.example.com/auth/login' -H 'Content-Type: application/json' --data-raw '{\"username\":\"tester\",\"password\":\"…\"}'",
  'prepare.captures': 'Giá trị cần bắt từ response',
  'prepare.var': 'Tên biến',
  'prepare.path': 'Path trong response',
  'prepare.pathAuto': 'để trống = tự tìm token',
  'prepare.addCapture': 'Thêm biến',
  'prepare.removeCapture': 'Xoá',
  'prepare.try': 'Chạy thử',
  'prepare.needAnalyze': 'Hãy Analyze tất cả các bước trước khi chạy thử.',
  'prepare.inject': 'Gắn Authorization: Bearer {{token}} vào curl chính',
  'prepare.useVars': 'Các bước sau và curl chính có thể dùng {{name}} ở bất kỳ đâu trong URL, header hay body.',
  'prepare.status.OK': 'Thành công',
  'prepare.status.FAILED': 'Lỗi',
  'prepare.status.SKIPPED': 'Bỏ qua',
  'prepare.httpTime': 'HTTP {code} · {ms} ms',
  'prepare.captured': 'Đã bắt',
  'prepare.foundAt': 'lấy từ {path}',
  'prepare.response': 'Response body',
  'prepare.truncated': '(1 MB đầu)',
  'prepare.tryFailed': 'Không chạy thử được',

  'curl.card': '1. Curl',
  'curl.help': 'Dán lệnh curl copy từ Chrome / Firefox DevTools ("Copy as cURL", bash hoặc cmd) hoặc Postman.',
  'curl.analyze': 'Analyze',
  'curl.shortcut': 'Ctrl+Enter',
  'curl.error': 'Không dùng được curl này',

  'request.card': '2. Request',
  'request.request': 'Request',
  'request.body': 'Body',
  'body.json': 'JSON',
  'body.none': 'Không có body',
  'body.unsupported': 'Không hỗ trợ',
  'request.unsupported': 'v1 chỉ hỗ trợ body JSON',
  'request.unsupportedDesc': 'Request này không chạy được.',
  'request.warnings': '{n} cảnh báo',
  'headers.title': 'Header ({n})',
  'headers.none': 'Không có header',
  'headers.name': 'Tên',
  'headers.value': 'Giá trị',
  'headers.showSensitive': 'Hiện giá trị nhạy cảm',
  'headers.replaced': 'sẽ thay bằng Bearer {{token}} khi chạy',
  'vars.title': 'Biến được dùng',
  'vars.missing': 'chưa có bước lấy token nào bắt biến này',

  'fields.card': '3. Field rules',
  'fields.summary': 'Field ({n}) · {required} Required · {unrequired} Unrequired',
  'fields.allRequired': 'Tất cả Required',
  'fields.allUnrequired': 'Tất cả Unrequired',
  'fields.empty': 'Body không có field nào',
  'fields.path': 'Field path',
  'fields.type': 'Kiểu',
  'fields.sample': 'Giá trị mẫu',
  'fields.rule': 'Rule',
}

export const messages: Record<Lang, Record<MessageKey, string>> = { en, vi }

/** Fills {name} slots; a slot without a param (such as {{token}}) is left as written. */
export function format(template: string, params?: Record<string, string | number>): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (slot, name: string) => (name in params ? String(params[name]) : slot))
}
