import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { ConfigProvider } from 'antd'
import enUS from 'antd/locale/en_US'
import viVN from 'antd/locale/vi_VN'
import { I18nContext, type I18n } from './context'
import { format, messages, type Lang } from './messages'

const STORAGE_KEY = 'caserunner.lang'

// The remembered choice, else the browser language (D22). Storage can be
// unavailable (private mode, blocked site data), so every access is guarded.
function initialLang(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'en' || saved === 'vi') return saved
  } catch {
    // fall through to the browser language
  }
  return navigator.language.toLowerCase().startsWith('vi') ? 'vi' : 'en'
}

/** Provides the UI language and the matching Ant Design locale. */
export default function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(initialLang)

  const setLang = useCallback((next: Lang) => {
    setLangState(next)
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // the choice then lasts for this page only
    }
  }, [])

  const value = useMemo<I18n>(
    () => ({ lang, setLang, t: (key, params) => format(messages[lang][key], params) }),
    [lang, setLang],
  )

  return (
    <I18nContext.Provider value={value}>
      <ConfigProvider locale={lang === 'vi' ? viVN : enUS}>{children}</ConfigProvider>
    </I18nContext.Provider>
  )
}
