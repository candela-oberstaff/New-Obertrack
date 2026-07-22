import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import Layout from '../components/layout/Layout'
import { ROUTES } from '../constants/routes'
import { AdminRoute, AuthRoute, ProtectedRoute, ReportsRoute, CustomerSuccessRoute, PlatformTechRoute, EmployerRoute, SupportRoute, SupportInboxRoute } from './guards'
import { LoadingScreen } from './LoadingScreen'
import { WALLET_ENABLED } from '../config/features'

const Login = lazy(() => import('../pages/Login'))
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
const SupportBoard = lazy(() => import('../pages/Tickets/SupportBoard'))
const TicketDetail = lazy(() => import('../pages/Tickets/TicketDetail'))
const WhatsAppTicketDetail = lazy(() => import('../pages/Tickets/WhatsAppTicketDetail'))
const RejectionReport = lazy(() => import('../pages/Tickets/RejectionReport'))
const InternalTicketDetail = lazy(() => import('../pages/Tickets/InternalTicketDetail'))
const EmailCampaigns = lazy(() => import('../pages/Email/EmailCampaigns'))
const EmpresaEmployees = lazy(() => import('../pages/Empresa/EmpresaEmployees'))
const ProfessionalsMap = lazy(() => import('../pages/ProfessionalsMap'))
const Incidents = lazy(() => import('../pages/Incidents'))
const Wallet = lazy(() => import('../pages/Wallet'))
const EmpresaEmployeeDetail = lazy(() => import('../pages/Empresa/EmployeeDetail'))
const Soporte = lazy(() => import('../pages/Soporte'))
const Papelera = lazy(() => import('../pages/Papelera'))
const AppSettings = lazy(() => import('../pages/AppSettings'))

export function AppRoutes() {
  return (
    <Suspense fallback={<LoadingScreen />}>
      <Routes>
        <Route path="/login" element={<AuthRoute><Login /></AuthRoute>} />
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
          <Route path="whatsapp" element={<AdminRoute><WhatsApp /></AdminRoute>} />
          <Route path="profile" element={<Profile />} />
          <Route path="soporte" element={<SupportRoute><Soporte /></SupportRoute>} />
          {/* Admin y Empresas: superadmin gestiona; CS consulta (el backend solo les permite GETs). */}
          <Route path="admin" element={<CustomerSuccessRoute><Admin /></CustomerSuccessRoute>} />
          <Route path="admin/users/:id" element={<CustomerSuccessRoute><AdminUserDetail /></CustomerSuccessRoute>} />
          <Route path="admin/tenants" element={<CustomerSuccessRoute><TenantsList /></CustomerSuccessRoute>} />
          <Route path="admin/tenants/:id" element={<CustomerSuccessRoute><TenantDetail /></CustomerSuccessRoute>} />
          <Route path="admin/tenants/:id/employees/:eid" element={<CustomerSuccessRoute><EmployeeDetail /></CustomerSuccessRoute>} />
          <Route path="admin/tools" element={<CustomerSuccessRoute><Tools /></CustomerSuccessRoute>} />
          <Route path="admin/email" element={<CustomerSuccessRoute><EmailCampaigns /></CustomerSuccessRoute>} />
          <Route path="admin/mapa" element={<AdminRoute><ProfessionalsMap /></AdminRoute>} />
          <Route path="admin/incidentes" element={<AdminRoute><Incidents /></AdminRoute>} />
          {WALLET_ENABLED && <Route path="wallet" element={<Wallet />} />}
          <Route path="admin/metrics" element={<PlatformTechRoute><Metrics /></PlatformTechRoute>} />
          <Route path="admin/audit" element={<PlatformTechRoute><AuditLogs /></PlatformTechRoute>} />
          <Route path="admin/settings" element={<AdminRoute><AppSettings /></AdminRoute>} />
          <Route path="papelera" element={<AdminRoute><Papelera /></AdminRoute>} />
          <Route path="empresa" element={<EmployerRoute><EmpresaEmployees /></EmployerRoute>} />
          <Route path="empresa/employees/:id" element={<EmployerRoute><EmpresaEmployeeDetail /></EmployerRoute>} />
          <Route path="novedades" element={<Tutoriales />} />
          <Route path="tutoriales" element={<Navigate to="/novedades" replace />} />
          <Route path="reports" element={<ReportsRoute><Reports /></ReportsRoute>} />
          {/* Roles y Grupos: no disponible para empresas en esta versión (solo superadmin). */}
          <Route path="roles-grupos" element={<AdminRoute><RolesGroups /></AdminRoute>} />
          <Route path="survey/:id" element={<SurveyViewer />} />
          <Route path="tickets" element={<AdminRoute><TicketsBoard /></AdminRoute>} />
          <Route path="tickets/soporte" element={<SupportInboxRoute><SupportBoard /></SupportInboxRoute>} />
          <Route path="tickets/report" element={<AdminRoute><RejectionReport /></AdminRoute>} />
          <Route path="tickets/internal/:id" element={<AdminRoute><InternalTicketDetail /></AdminRoute>} />
          <Route path="tickets/wa/:id" element={<SupportInboxRoute><WhatsAppTicketDetail /></SupportInboxRoute>} />
          <Route path="tickets/:id" element={<AdminRoute><TicketDetail /></AdminRoute>} />
        </Route>
      </Routes>
    </Suspense>
  )
}
