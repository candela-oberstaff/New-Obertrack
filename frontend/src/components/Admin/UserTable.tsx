import { User } from '../../types'
import styles from './Admin.module.css'
import { Pencil, Ban, CheckCircle2, Trash2 } from 'lucide-react'

interface UserTableProps {
  users: User[]
  employers: User[]
  onEdit: (user: User) => void
  onToggleStatus: (user: User) => void
  onDelete: (id: number) => void
  getRoleColor: (userType: string, isManager: boolean, isSuperadmin: boolean) => string
  getRoleLabel: (userType: string, isManager: boolean, isSuperadmin: boolean) => string
}

export function UserTable({
  users,
  employers,
  onEdit,
  onToggleStatus,
  onDelete,
  getRoleColor,
  getRoleLabel
}: UserTableProps) {
  return (
    <div className={styles['admin-content']}>
      <div className={styles['admin-section-header'] || 'admin-section-header'}>
        <h3>Todos los Usuarios</h3>
      </div>
      <div className={styles['users-table']}>
        <table>
          <thead>
            <tr>
              <th>Nombre</th>
              <th>Email</th>
              <th>Tipo</th>
              <th>Empresa</th>
              <th>Estado</th>
              <th>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => {
              const employer = employers.find(e => e.id === u.empleador_id)
              const displayCompany = u.user_type === 'empleador'
                ? (u.company_name || '-')
                : (employer?.company_name || employer?.name || '-')
              return (
                <tr key={u.id}>
                  <td>{u.name}</td>
                  <td>{u.email}</td>
                  <td>
                    <span
                      className={styles['role-badge']}
                      style={{ background: getRoleColor(u.user_type, u.is_manager || false, u.is_superadmin || false) }}
                    >
                      {getRoleLabel(u.user_type, u.is_manager || false, u.is_superadmin || false)}
                    </span>
                  </td>
                  <td>{displayCompany}</td>
                  <td>
                    <span className={`${styles['status-pill']} ${u.is_active !== false ? styles['active'] : styles['inactive']}`}>
                      {u.is_active !== false ? 'Activo' : 'Inactivo'}
                    </span>
                  </td>
                  <td>
                    <div className={styles['action-buttons']}>
                      <button
                        className={styles['btn-icon']}
                        onClick={() => onEdit(u)}
                        title="Editar"
                      >
                        <Pencil size={16} />
                      </button>
                      <button
                        className={styles['btn-icon']}
                        onClick={() => onToggleStatus(u)}
                        title={u.is_active !== false ? 'Desactivar' : 'Activar'}
                      >
                        {u.is_active !== false ? <Ban size={16} /> : <CheckCircle2 size={16} />}
                      </button>
                      <button
                        className={`${styles['btn-icon']} ${styles['danger']}`}
                        onClick={() => onDelete(u.id)}
                        title="Eliminar"
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
            {users.length === 0 && (
              <tr>
                 <td colSpan={6} className={styles['no-data'] || 'no-data'}>No hay usuarios registrados</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
