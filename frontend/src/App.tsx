import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import Login from './pages/Login'
import Register from './pages/Register'
import Layout from './components/layout/Layout'
import Dashboard from './pages/Dashboard'
import Tasks from './pages/Tasks'
import WorkHours from './pages/WorkHours'
import Reports from './pages/Reports'
import Chat from './pages/Chat'
import Profile from './pages/Profile'
import Admin from './pages/Admin'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <div className="loading">Cargando...</div>
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

function App() {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <div className="loading">Cargando...</div>
  }

  return (
    <Routes>
      <Route 
        path="/login" 
        element={user ? <Navigate to="/dashboard" replace /> : <Login />} 
      />
      <Route 
        path="/register" 
        element={user ? <Navigate to="/dashboard" replace /> : <Register />} 
      />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<Dashboard />} />
        <Route path="tasks" element={<Tasks />} />
        <Route path="work-hours" element={<WorkHours />} />
        <Route path="reports" element={<Reports />} />
        <Route path="chat" element={<Chat />} />
        <Route path="profile" element={<Profile />} />
        <Route path="admin" element={<Admin />} />
      </Route>
    </Routes>
  )
}

export default App
