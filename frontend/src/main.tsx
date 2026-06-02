import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider } from './context/AuthContext'
import { NotificationProvider } from './context/NotificationContext'
import { ConfirmProvider } from './components/ui/ConfirmProvider'
import Toast from './components/Toast'
import App from './App'
import { queryClient } from './config/queryClient'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <BrowserRouter>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <NotificationProvider>
          <ConfirmProvider>
            <Toast />
            <App />
          </ConfirmProvider>
        </NotificationProvider>
      </AuthProvider>
    </QueryClientProvider>
  </BrowserRouter>
)
