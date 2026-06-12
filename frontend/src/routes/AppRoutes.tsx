import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import Layout from '../components/layout/Layout'
import { ROUTES } from '../constants/routes'
import { AuthRoute, ProtectedRoute, ReportsRoute, CustomerSuccessRoute, PlatformTechRoute } from './guards'
import { LoadingScreen } from './LoadingScreen'

const Login = lazy(() => import('../pages/Login'))
const Register = lazy(() => import('../pages/Register'))
const ForgotPassword = lazy(() => import('../pages/ForgotPassword'))
const ResetPassword = lazy(() => import('../pages/ResetPassword'))
const Dashboard = lazy(() => import('../pages/Dashboard'))
const Tasks = lazy(() => import('../pages/Tasks'))
const WorkHours = lazy(() => import('../pages/WorkHours'))
const Reports = lazy(() => import('../pages/Reports'))
const SlackChat = lazy(() => import('../pages/SlackChat'))
const WhatsApp = lazy(() => import('../pages/WhatsApp'))
const Profile = lazy(() => import('../pages/Profile'))
const Admin = lazy(() => import('../pages/Admin'))
const AdminUserDetail = lazy(() => import('../pages/AdminUserDetail'))
const TenantsList = lazy(() => import('../pages/Tenants/TenantsList'))
const TenantDetail = lazy(() => import('../pages/Tenants/TenantDetail'))
const EmployeeDetail = lazy(() => import('../pages/Tenants/EmployeeDetail'))
const Tools = lazy(() => import('../components/Admin/Tools'))
const Metrics = lazy(() => import('../pages/Metrics'))
const AuditLogs = lazy(() => import('../pages/AuditLogs'))
const Tutoriales = lazy(() => import('../pages/Tutoriales'))
const RolesGroups = lazy(() => import('../pages/RolesGroups'))
const SurveyViewer = lazy(() => import('../pages/SurveyViewer'))
const TicketsBoard = lazy(() => import('../pages/Tickets/TicketsBoard'))
const TicketDetail = lazy(() => import('../pages/Tickets/TicketDetail'))
const RejectionReport = lazy(() => import('../pages/Tickets/RejectionReport'))
const InternalTicketDetail = lazy(() => import('../pages/Tickets/InternalTicketDetail'))

export function AppRoutes() {
  return (
    <Suspense fallback={<LoadingScreen />}>
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
          <Route index element={<Navigate to={ROUTES.dashboard} replace />} />
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="tasks" element={<Tasks />} />
          <Route path="work-hours" element={<WorkHours />} />
          <Route path="chat" element={<SlackChat />} />
          <Route path="whatsapp" element={<CustomerSuccessRoute><WhatsApp /></CustomerSuccessRoute>} />
          <Route path="profile" element={<Profile />} />
          {/* Admin y Empresas: superadmin gestiona; CS consulta (el backend solo les permite GETs). */}
          <Route path="admin" element={<CustomerSuccessRoute><Admin /></CustomerSuccessRoute>} />
          <Route path="admin/users/:id" element={<CustomerSuccessRoute><AdminUserDetail /></CustomerSuccessRoute>} />
          <Route path="admin/tenants" element={<CustomerSuccessRoute><TenantsList /></CustomerSuccessRoute>} />
          <Route path="admin/tenants/:id" element={<CustomerSuccessRoute><TenantDetail /></CustomerSuccessRoute>} />
          <Route path="admin/tenants/:id/employees/:eid" element={<CustomerSuccessRoute><EmployeeDetail /></CustomerSuccessRoute>} />
          <Route path="admin/tools" element={<CustomerSuccessRoute><Tools /></CustomerSuccessRoute>} />
          <Route path="admin/metrics" element={<PlatformTechRoute><Metrics /></PlatformTechRoute>} />
          <Route path="admin/audit" element={<PlatformTechRoute><AuditLogs /></PlatformTechRoute>} />
          <Route path="tutoriales" element={<Tutoriales />} />
          <Route path="reports" element={<ReportsRoute><Reports /></ReportsRoute>} />
          <Route path="roles-grupos" element={<ReportsRoute><RolesGroups /></ReportsRoute>} />
          <Route path="survey/:id" element={<SurveyViewer />} />
          <Route path="tickets" element={<CustomerSuccessRoute><TicketsBoard /></CustomerSuccessRoute>} />
          <Route path="tickets/report" element={<CustomerSuccessRoute><RejectionReport /></CustomerSuccessRoute>} />
          <Route path="tickets/internal/:id" element={<CustomerSuccessRoute><InternalTicketDetail /></CustomerSuccessRoute>} />
          <Route path="tickets/:id" element={<CustomerSuccessRoute><TicketDetail /></CustomerSuccessRoute>} />
        </Route>
      </Routes>
    </Suspense>
  )
}
