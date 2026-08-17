import { useAuth } from '../context/AuthContext'
import { OrgChartPanel } from '../components/OrgChart/OrgChartPanel'

/**
 * Organigrama propio: la empresa ve y mantiene el suyo completo; un supervisor
 * ve su rama, con él como raíz.
 *
 * La misma pantalla sirve a los dos porque el recorte lo hace el backend a
 * partir de quién pide: aquí solo cambia el texto que explica qué se está
 * viendo, para que nadie crea que le falta gente.
 */
export default function Organigrama() {
  const { user } = useAuth()
  const isEmployer = user?.user_type === 'empleador'

  return (
    <div style={{ padding: '24px 32px' }}>
      <h1 style={{ fontSize: 26, fontWeight: 800, color: '#0f172a', margin: '0 0 4px' }}>
        Organigrama
      </h1>
      <p style={{ color: '#64748b', fontSize: 14, margin: '0 0 20px' }}>
        {isEmployer
          ? 'Estructura de tu empresa. Arrastra a una persona sobre otra para cambiar su manager.'
          : 'Tu estructura a cargo. Arrastra a una persona sobre otra para reorganizar tu equipo.'}
      </p>

      <OrgChartPanel
        editable
        hint={isEmployer
          ? 'Arrastra a una persona sobre otra para cambiar su manager. Se lleva a su equipo con ella. Haz clic en alguien para abrir su ficha.'
          : 'Solo se muestra tu rama. Arrastra dentro de ella para reorganizar; se lleva su equipo con ella.'}
        // Solo el empleador tiene una ficha que abrir (/empresa/employees/:id).
        // El supervisor no llega a ninguna pantalla de perfil ajeno, así que sus
        // tarjetas se quedan sin enlace en vez de llevar a un "no autorizado".
        profileHref={isEmployer ? p => `/empresa/employees/${p.user_id}` : undefined}
      />
    </div>
  )
}
