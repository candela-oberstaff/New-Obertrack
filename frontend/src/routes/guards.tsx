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

export function ReportsRoute({ children }: RouteGuardProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return <Navigate to={ROUTES.login} replace />
  }

  if (!user.is_superadmin && user.user_type !== 'empleador') {
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
