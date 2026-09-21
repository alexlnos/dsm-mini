import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { setAuthToken } from './api'
import { initTelegram } from './telegram'
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
          <h1>Откройте через Telegram</h1>
          <p>
            Приложение работает внутри бота: Telegram передаёт подпись, по которой
            сервис узнаёт, кто вы. По прямой ссылке в браузере оно не откроется.
          </p>
        </div>
      )}
    </StrictMode>,
  )
}
