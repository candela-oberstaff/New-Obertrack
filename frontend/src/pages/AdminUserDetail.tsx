import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, UserX } from 'lucide-react'
import { userService } from '../services/api'
import type { User } from '../types'
import Avatar from '../components/Common/Avatar'
import styles from './AdminUserDetail.module.css'

export default function AdminUserDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const load = async () => {
      setIsLoading(true)
      try {
        const data = await userService.getById(Number(id))
        setUser(data)
        setError(null)
      } catch {
        setError('No se pudo cargar el usuario')
      } finally {
        setIsLoading(false)
      }
    }
    load()
  }, [id])

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Cargando usuario...</p>
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

  const rol = user.is_superadmin ? 'superadmin' : user.is_manager ? 'manager' : user.user_type

  const fields: { label: string; value: React.ReactNode }[] = [
    { label: 'ID', value: user.id },
    { label: 'Rol', value: rol },
    { label: 'Empresa', value: user.company_name || '—' },
    { label: 'Cargo', value: user.job_title || '—' },
    { label: 'Empleador ID', value: user.empleador_id || '—' },
    { label: 'Manager ID', value: user.manager_id || '—' },
    { label: 'Teléfono', value: user.phone_number || '—' },
    { label: 'País', value: user.country || '—' },
    { label: 'Ciudad', value: user.city || '—' },
    { label: 'Ubicación', value: user.location || '—' },
    { label: 'Registrado', value: user.created_at ? new Date(user.created_at).toLocaleString('es-ES') : '—' },
    { label: 'Actualizado', value: user.updated_at ? new Date(user.updated_at).toLocaleString('es-ES') : '—' },
  ]

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
            <span className={styles.tag}>{user.user_type}</span>
            {user.is_superadmin && <span className={`${styles.tag} ${styles.tagAdmin}`}>superadmin</span>}
            {user.is_manager && <span className={`${styles.tag} ${styles.tagManager}`}>manager</span>}
          </div>
        </div>
      </div>

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
