import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider } from './context/AuthContext'
import { NotificationProvider } from './context/NotificationContext'
import { ConfirmProvider } from './components/ui/ConfirmProvider'
import { PromptProvider } from './components/ui/PromptProvider'
import Toast from './components/Toast'
import App from './App'
import { queryClient } from './config/queryClient'
import './index.css'
import { bootEmbedSession } from './embed/session'

// Antes de montar nada: si es la vista embebida en el CRM, toma el token del
// hash y lo borra de la URL, para que AuthProvider arranque ya con él.
bootEmbedSession()

createRoot(document.getElementById('root')!).render(
  <BrowserRouter>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <NotificationProvider>
          <ConfirmProvider>
            <PromptProvider>
              <Toast />
              <App />
            </PromptProvider>
          </ConfirmProvider>
        </NotificationProvider>
      </AuthProvider>
    </QueryClientProvider>
  </BrowserRouter>
)
