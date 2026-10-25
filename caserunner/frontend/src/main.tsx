import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App as AntApp, ConfigProvider } from 'antd'
import App from './App'
import LanguageProvider from './i18n/LanguageProvider'
import { theme } from './theme'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <LanguageProvider>
      <ConfigProvider theme={theme}>
        <AntApp>
          <App />
        </AntApp>
      </ConfigProvider>
    </LanguageProvider>
  </StrictMode>,
)
