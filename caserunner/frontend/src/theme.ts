import type { ThemeConfig } from 'antd'

/** Look of the whole tool: one accent color, soft surfaces, compact tables. */
export const theme: ThemeConfig = {
  token: {
    colorPrimary: '#4f46e5',
    colorBgLayout: '#f4f6fb',
    borderRadius: 8,
    fontSize: 14,
  },
  components: {
    Layout: { headerBg: '#111827', headerHeight: 56, headerPadding: '0 24px' },
    Card: { headerFontSize: 15 },
    Collapse: { headerBg: '#ffffff', contentBg: '#ffffff' },
    Table: { headerBg: '#f8fafc', cellPaddingBlockSM: 6 },
  },
}
