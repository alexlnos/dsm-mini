import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { setAuthToken } from './api'
import { initTelegram } from './telegram'
import { t } from './i18n'
import './styles.css'

const initData = initTelegram()
setAuthToken(initData)

const root = document.getElementById('root')
if (root) {
  createRoot(root).render(
    <StrictMode>
      {initData ? (
        <App />
      ) : (
        <div className="standalone">
          <h1>{t('standalone.title')}</h1>
          <p>{t('standalone.text')}</p>
        </div>
      )}
    </StrictMode>,
  )
}
