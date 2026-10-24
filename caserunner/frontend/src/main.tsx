import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App as AntApp } from 'antd'
import App from './App'
import LanguageProvider from './i18n/LanguageProvider'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <LanguageProvider>
      <AntApp>
        <App />
      </AntApp>
    </LanguageProvider>
  </StrictMode>,
)
