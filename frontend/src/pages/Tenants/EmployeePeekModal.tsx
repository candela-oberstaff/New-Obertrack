import { useNavigate } from 'react-router-dom'
import { ArrowRight, Ban, CheckCircle2, MessageSquare, RefreshCw, User as UserIcon } from 'lucide-react'
import { useEmployeeTracking, useTenantDetail } from '../../hooks'
import { Modal, Button, Skeleton } from '../../components/ui'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import Avatar from '../../components/Common/Avatar'
import { EmployeeFicha, EmployeeKpis } from './EmployeeFicha'
import styles from './Tenants.module.css'
import peek from './EmployeePeek.module.css'

interface Props {
  employeeId: number
  tenantId: number
  canManage: boolean
  onClose: () => void
  /** Lleva a la ficha completa conservando el orden de la tabla para el pager. */
  onOpenFull: (employeeId: number) => void
}

/**
 * Vistazo rápido a un profesional desde la plantilla de la empresa. Existe
 * porque la pregunta habitual sobre alguien de la lista —quién es, quién lo
 * supervisa, cuándo se le vio por última vez— se responde en cinco segundos y
 * no merece perder la tabla, sus filtros y su página.
 *
 * Lee del MISMO hook que la ficha completa, así que abrirla después de este
 * vistazo no vuelve a pedir nada: la respuesta ya está en caché.
 */
export function EmployeePeekModal({ employeeId, tenantId, canManage, onClose, onOpenFull }: Props) {
  const navigate = useNavigate()
  const confirm = useConfirm()
  const { tracking, isLoading, error, toggleStatus, resetPassword } = useEmployeeTracking(employeeId)
  const { employees } = useTenantDetail(tenantId)

  const user = tracking?.user
  const manager = user?.manager_id ? employees.find(e => e.id === user.manager_id) : null
  const phone = user?.phone_number?.trim()
  const waNumber = phone?.replace(/\D/g, '')

  const handleReset = async () => {
    const ok = await confirm({
      title: 'Resetear contraseña',
      message: '¿Resetear la contraseña a "temporary123"?',
      confirmLabel: 'Resetear',
      variant: 'primary',
    })
    if (ok) await resetPassword('temporary123')
  }

  return (
    <Modal isOpen onClose={onClose} size="lg" title={user?.name || 'Profesional'}>
      {isLoading ? (
        <div className={peek.body}>
          {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} height={70} radius={12} />)}
        </div>
      ) : error || !user ? (
        <div className={styles.empty}>
          <UserIcon size={40} />
          <p>{error || 'No se pudo cargar el profesional'}</p>
        </div>
      ) : (
        <div className={peek.body}>
          {/* El nombre no se repite: ya está en la barra del modal, justo
              encima. Aquí va lo que esa barra no puede decir. */}
          <div className={peek.identity}>
            <Avatar src={user.avatar} name={user.name} size="lg" />
            <div className={peek.identityMeta}>
              <span className={peek.email}>{user.email}</span>
              <div className={peek.badges}>
                <span className={`${styles.badge} ${user.is_active ? styles.badgeActive : styles.badgeSuspended}`}>
                  {user.is_active ? 'Activo' : 'Inactivo'}
                </span>
                <span className={styles.typeBadge}>{user.is_manager ? 'manager' : user.user_type}</span>
              </div>
            </div>
          </div>

          <EmployeeFicha user={user} managerName={manager?.name} compact />
          <EmployeeKpis summary={tracking?.summary} compact />

          {/* Las acciones del vistazo son las que no necesitan contexto: escribir,
              devolver el acceso, quitarlo. Todo lo que exige leer antes (el
              expediente, las gestiones, los tickets) vive en la ficha completa. */}
          {/* Todas las acciones son el mismo primitivo, así comparten alto y
              radio. El de WhatsApp es un <a> de verdad (abre otra pestaña), y
              toma prestadas sus clases para no ser el único distinto. */}
          <div className={peek.actions}>
            {waNumber && (
              <a
                className="ui-btn ui-btn--secondary ui-btn--md"
                href={`https://wa.me/${waNumber}`}
                target="_blank"
                rel="noreferrer noopener"
                title={`Escribir a ${phone} por WhatsApp`}
              >
                <MessageSquare size={16} /> WhatsApp
              </a>
            )}
            {canManage && (
              <>
                <Button variant="secondary" onClick={handleReset} leftIcon={<RefreshCw size={16} />}>
                  Resetear contraseña
                </Button>
                {user.is_active ? (
                  <Button variant="danger" onClick={toggleStatus} leftIcon={<Ban size={16} />}>
                    Desactivar
                  </Button>
                ) : (
                  <Button variant="success" onClick={toggleStatus} leftIcon={<CheckCircle2 size={16} />}>
                    Activar
                  </Button>
                )}
              </>
            )}
            <Button
              variant="primary"
              rightIcon={<ArrowRight size={15} />}
              onClick={() => { onOpenFull(employeeId); navigate(`/admin/tenants/${tenantId}/employees/${employeeId}`) }}
            >
              Ver ficha completa
            </Button>
          </div>
        </div>
      )}
    </Modal>
  )
}
