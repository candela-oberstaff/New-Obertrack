import { useQuery, useQueryClient } from '@tanstack/react-query'
import { orgChartService, userService } from '../../services/api'
import { OrgChart } from './OrgChart'

interface OrgChartPanelProps {
  /** Solo el superadmin la indica; el resto usa la suya. */
  companyId?: number
  /** Sin esto el árbol se dibuja pero no se reordena. */
  editable?: boolean
  hint?: string
}

/**
 * Resuelve los datos y la mutación del organigrama, para que las tres pantallas
 * que lo muestran (ficha de empresa, panel del empleador y panel del supervisor)
 * no repitan el cableado.
 *
 * La mutación es POST /users/:id/assign-manager, que ya existía y ya está
 * autorizada para los tres públicos: el backend acota al supervisor a su árbol y
 * rechaza los círculos, así que aquí no hace falta ninguna comprobación de
 * permisos más.
 */
export function OrgChartPanel({ companyId, editable = false, hint }: OrgChartPanelProps) {
  const qc = useQueryClient()
  const queryKey = ['org-chart', companyId ?? 'me']

  const { data: people = [], isLoading, error } = useQuery({
    queryKey,
    queryFn: () => orgChartService.get(companyId),
  })

  const reassign = async (userId: number, newManagerId: number | null) => {
    // El nivel del destino se deduce de lo que acaba de recibir, que es como se
    // construye un organigrama de verdad:
    //
    //  - recibe a alguien y no era manager  → pasa a manager
    //  - recibe a un MANAGER                → pasa a supervisor, porque tener
    //    managers a cargo es exactamente la definición del rol
    //
    // Sin esto el backend respondía "el usuario seleccionado no es manager" y
    // dejaba al usuario sin salida. Es el mismo criterio que ya aplica el
    // importador cuando alguien aparece en la columna reporta_a.
    //
    // No hay degradación automática al revés: quitarle managers a alguien no le
    // retira el rol, igual que en el resto de la aplicación.
    if (newManagerId != null) {
      const target = people.find(p => p.user_id === newManagerId)
      const moved = people.find(p => p.user_id === userId)
      const needsSupervisor = !!moved?.is_manager && !target?.is_supervisor
      if (target && (needsSupervisor || !target.is_manager)) {
        await userService.promoteToManager(
          newManagerId,
          true,
          needsSupervisor ? true : undefined,
        )
      }
    }
    await userService.assignToManager(userId, newManagerId, companyId)
    // El árbol se redibuja desde el servidor y no en memoria: así lo que se ve
    // es lo que quedó guardado, incluido cualquier ajuste que haya hecho el
    // backend (espejos del empleo, manager principal).
    await qc.invalidateQueries({ queryKey })
  }

  if (isLoading) return <p style={{ color: '#94a3b8', fontSize: 13 }}>Cargando organigrama…</p>
  if (error) {
    return (
      <p style={{ color: '#b91c1c', fontSize: 13, fontWeight: 600 }}>
        {(error as any)?.response?.data?.error ?? 'No se pudo cargar el organigrama.'}
      </p>
    )
  }

  return (
    <OrgChart
      people={people}
      hint={hint}
      onReassign={editable ? reassign : undefined}
    />
  )
}
