import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, UserX, Power, KeyRound, Shield } from 'lucide-react'
import { userService, adminService } from '../services/api'
import type { User } from '../types'
import Avatar from '../components/Common/Avatar'
import { Skeleton } from '../components/ui'
import styles from './AdminUserDetail.module.css'

export default function AdminUserDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionMsg, setActionMsg] = useState<string | null>(null)
  const [empresaName, setEmpresaName] = useState('')
  const [managerName, setManagerName] = useState('')

  const load = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await userService.getById(Number(id))
      setUser(data)
      setError(null)
      // Resolve related names (employer / manager) for professionals.
      setEmpresaName(''); setManagerName('')
      if (data.user_type === 'profesional') {
        if (data.empleador_id) {
          userService.getById(data.empleador_id).then(e => setEmpresaName(e?.company_name || e?.name || '')).catch(() => {})
        }
        if (data.manager_id) {
          userService.getById(data.manager_id).then(m => setManagerName(m?.name || '')).catch(() => {})
        }
      }
    } catch {
      setError('No se pudo cargar el usuario')
    } finally {
      setIsLoading(false)
    }
  }, [id])

  useEffect(() => { load() }, [load])

  const toggleActive = async () => {
    if (!user) return
    setBusy(true); setActionMsg(null)
    try {
      await adminService.updateUser(user.id, { is_active: !user.is_active })
      await load()
    } catch { setActionMsg('No se pudo cambiar el estado.') } finally { setBusy(false) }
  }

  const promote = async () => {
    if (!user) return
    setBusy(true); setActionMsg(null)
    try {
      await adminService.updateUser(user.id, { is_manager: true })
      await load()
    } catch { setActionMsg('No se pudo promover.') } finally { setBusy(false) }
  }

  const resetPass = async () => {
    if (!user) return
    setBusy(true); setActionMsg(null)
    try {
      const temp = 'Temporal' + Math.floor(1000 + (user.id * 7) % 9000)
      await adminService.resetPassword(user.id, temp)
      setActionMsg(`Contraseña reseteada. Temporal: ${temp}`)
    } catch { setActionMsg('No se pudo resetear la contraseña.') } finally { setBusy(false) }
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', padding: '1.5rem' }}>
          <Skeleton height={120} radius={16} />
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}>
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} height={140} radius={16} />)}
          </div>
        </div>
      </div>
    )
  }

  if (error || !user) {
    return (
      <div className={styles.page}>
        <button className={styles.backBtn} onClick={() => navigate('/admin')}>
          <ArrowLeft size={18} /> Volver
        </button>
        <div className={styles.empty}>
          <UserX size={40} />
          <p>{error || 'Usuario no encontrado'}</p>
        </div>
      </div>
    )
  }

  const rol = user.is_superadmin ? 'Superadmin' : user.is_manager ? 'Manager' : user.user_type

  // Fields common to every user type.
  const common: { label: string; value: React.ReactNode }[] = [
    { label: 'Teléfono', value: user.phone_number || '—' },
    { label: 'País', value: user.country || '—' },
    { label: 'Ciudad', value: user.city || '—' },
    { label: 'Ubicación', value: user.location || '—' },
    { label: 'Registrado', value: user.created_at ? new Date(user.created_at).toLocaleString('es-ES') : '—' },
    { label: 'Actualizado', value: user.updated_at ? new Date(user.updated_at).toLocaleString('es-ES') : '—' },
  ]

  // Type-specific fields: only show what's relevant for each user type.
  let specific: { label: string; value: React.ReactNode }[] = []
  if (user.user_type === 'empleador') {
    specific = [{ label: 'Empresa', value: user.company_name || '—' }]
  } else if (user.user_type === 'profesional') {
    specific = [
      { label: 'Cargo', value: user.job_title || '—' },
      { label: 'Empresa', value: empresaName || '—' },
      { label: 'Manager', value: managerName || 'Sin asignar' },
    ]
  }

  const fields = [...specific, ...common]

  return (
    <div className={styles.page}>
      <button className={styles.backBtn} onClick={() => navigate('/admin')}>
        <ArrowLeft size={18} /> Usuarios
      </button>

      <div className={styles.header}>
        <Avatar src={user.avatar} name={user.name} size="xl" />
        <div>
          <div className={styles.titleRow}>
            <h1>{user.name}</h1>
            <span className={`${styles.status} ${user.is_active ? styles.active : styles.inactive}`}>
              {user.is_active ? 'Activo' : 'Inactivo'}
            </span>
          </div>
          <p className={styles.email}>{user.email}</p>
          <div className={styles.tags}>
            <span className={`${styles.tag} ${user.is_superadmin ? styles.tagAdmin : ''}`}>{rol}</span>
            {user.is_manager && !user.is_superadmin && <span className={`${styles.tag} ${styles.tagManager}`}>Manager</span>}
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', margin: '0 0 1rem' }}>
        <button onClick={toggleActive} disabled={busy}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
          <Power size={15} /> {user.is_active ? 'Desactivar' : 'Activar'}
        </button>
        <button onClick={resetPass} disabled={busy}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
          <KeyRound size={15} /> Resetear contraseña
        </button>
        {!user.is_manager && !user.is_superadmin && (
          <button onClick={promote} disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
            <Shield size={15} /> Promover a manager
          </button>
        )}
      </div>
      {actionMsg && (
        <div style={{ margin: '0 0 1rem', padding: '0.6rem 0.9rem', borderRadius: '8px', background: 'rgba(16,185,129,0.1)', color: '#059669', fontSize: '0.85rem', fontWeight: 600 }}>{actionMsg}</div>
      )}

      <div className={styles.card}>
        <h3>Información</h3>
        <div className={styles.grid}>
          {fields.map(f => (
            <div key={f.label} className={styles.row}>
              <span className={styles.label}>{f.label}</span>
              <span className={styles.value}>{f.value}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
