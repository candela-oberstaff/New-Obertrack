import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { AUTH_REDIRECT_PATH, ROUTES, UNAUTHORIZED_REDIRECT_PATH } from '../constants/routes'
import { useAuth } from '../context/AuthContext'
import { LoadingScreen } from './LoadingScreen'

type RouteGuardProps = {
  children: ReactNode
}

export function ProtectedRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  return <>{children}</>
}

export function AdminRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (!user.is_superadmin) {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

// Administración de plataforma: superadmins y Customer Success, que trabaja con
// el mismo alcance. Se separa de AdminRoute —que sigue siendo solo-superadmin—
// porque cuatro pantallas quedan fuera de CS: Papelera, Auditoría,
// Configuración y Novedades.
export function PlatformAdminRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (!user.is_superadmin && user.user_type !== 'customer_success') {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

// Novedades: visible para todos MENOS Customer Success, así que la exclusión se
// escribe al revés que las demás.
export function NovedadesRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (!user.is_superadmin && user.user_type === 'customer_success') {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

export function ReportsRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (!user.is_superadmin && user.user_type !== 'empleador' && user.user_type !== 'customer_success') {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

// Soporte técnico de plataforma: superadmins y analistas de IT.
export function PlatformTechRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (!user.is_superadmin && user.user_type !== 'analista_it') {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

export function CustomerSuccessRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (!user.is_superadmin && user.user_type !== 'customer_success') {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

export function SupportInboxRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  const allowed =
    user.is_superadmin ||
    user.user_type === 'customer_success' ||
    user.user_type === 'analista_it'
  if (!allowed) {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

// Módulo de empresa del EMPLEADOR: gestiona a sus empleados acotado a su tenant.
// Solo cuentas empleador (no superadmin, no otros tipos).
export function EmployerRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (user.user_type !== 'empleador') {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

// Organigrama propio: lo mantiene la empresa y lo consulta/reordena también el
// supervisor sobre su rama. El backend acota lo que cada uno ve y puede mover;
// esta guarda solo evita ofrecer la pantalla a quien no tiene nada que hacer en
// ella (un profesional sin gente a cargo).
export function OrgChartRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (user.user_type !== 'empleador' && !user.is_supervisor) {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

export function SupportRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  const canOpen =
    !user.is_superadmin &&
    (user.user_type === 'profesional' || user.user_type === 'empleador')
  if (!canOpen) {
    return <Navigate to={UNAUTHORIZED_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}

export function AuthRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (user) {
    return <Navigate to={AUTH_REDIRECT_PATH} replace />
  }

  return <>{children}</>
}
