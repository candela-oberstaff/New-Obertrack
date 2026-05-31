import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import Layout from './components/layout/Layout'
import { lazy, Suspense } from 'react'

const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'))
const ResetPassword = lazy(() => import('./pages/ResetPassword'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const Tasks = lazy(() => import('./pages/Tasks'))
const WorkHours = lazy(() => import('./pages/WorkHours'))
const Reports = lazy(() => import('./pages/Reports'))
const SlackChat = lazy(() => import('./pages/SlackChat'))
const GoogleChat = lazy(() => import('./pages/GoogleChat'))
const WhatsApp = lazy(() => import('./pages/WhatsApp'))
const Profile = lazy(() => import('./pages/Profile'))
const Admin = lazy(() => import('./pages/Admin'))
const UserDetail = lazy(() => import('./pages/UserDetail'))
const AdminUserDetail = lazy(() => import('./pages/AdminUserDetail'))
const TenantsList = lazy(() => import('./pages/Tenants/TenantsList'))
const TenantDetail = lazy(() => import('./pages/Tenants/TenantDetail'))
const EmployeeDetail = lazy(() => import('./pages/Tenants/EmployeeDetail'))
const Tools = lazy(() => import('./components/Admin/Tools'))
const Metrics = lazy(() => import('./pages/Metrics'))
const Tutoriales = lazy(() => import('./pages/Tutoriales'))
const SurveyViewer = lazy(() => import('./pages/SurveyViewer'))
const TicketsBoard = lazy(() => import('./pages/Tickets/TicketsBoard'))
const TicketDetail = lazy(() => import('./pages/Tickets/TicketDetail'))

function Loading() {
  return (
    <div style={{ 
      display: 'flex', 
      justifyContent: 'center', 
      alignItems: 'center', 
      height: '100vh',
      color: '#64748b'
    }}>
      Cargando...
    </div>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <Loading />
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <Loading />
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  if (!user.is_superadmin) {
    return <Navigate to="/dashboard" replace />
  }

  return <>{children}</>
}

function ReportsRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <Loading />
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  if (!user.is_superadmin && user.user_type !== 'empleador') {
    return <Navigate to="/dashboard" replace />
  }

  return <>{children}</>
}

function AuthRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <Loading />
  }

  if (user) {
    return <Navigate to="/dashboard" replace />
  }

  return <>{children}</>
}

function App() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/login" element={<AuthRoute><Login /></AuthRoute>} />
        <Route path="/register" element={<AuthRoute><Register /></AuthRoute>} />
        <Route path="/forgot-password" element={<AuthRoute><ForgotPassword /></AuthRoute>} />
        <Route path="/reset-password" element={<AuthRoute><ResetPassword /></AuthRoute>} />
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
          <Route path="chat" element={<SlackChat />} />
          <Route path="google-chat" element={<GoogleChat />} />
          <Route path="whatsapp" element={<WhatsApp />} />
          <Route path="profile" element={<Profile />} />
          <Route path="admin" element={<AdminRoute><Admin /></AdminRoute>} />
          <Route path="admin/users/:id" element={<AdminRoute><UserDetail /></AdminRoute>} />
          <Route path="admin/users/:id" element={<AdminRoute><AdminUserDetail /></AdminRoute>} />
          <Route path="admin/tenants" element={<AdminRoute><TenantsList /></AdminRoute>} />
          <Route path="admin/tenants/:id" element={<AdminRoute><TenantDetail /></AdminRoute>} />
          <Route path="admin/tenants/:id/employees/:eid" element={<AdminRoute><EmployeeDetail /></AdminRoute>} />
          <Route path="admin/tools" element={<AdminRoute><Tools /></AdminRoute>} />
          <Route path="admin/metrics" element={<AdminRoute><Metrics /></AdminRoute>} />
          <Route path="tutoriales" element={<Tutoriales />} />
          <Route path="reports" element={<ReportsRoute><Reports /></ReportsRoute>} />
          <Route path="survey/:id" element={<SurveyViewer />} />
          <Route path="tickets" element={<TicketsBoard />} />
          <Route path="tickets/:id" element={<TicketDetail />} />
        </Route>
      </Routes>
    </Suspense>
  )
}

export default App
