import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Search, Eye, UserPlus, Copy, Check } from 'lucide-react'
import { userService, employerService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import Avatar from '../../components/Common/Avatar'
import { Modal, Button, Skeleton } from '../../components/ui'
import styles from '../../components/Admin/Admin.module.css'

// Lista de profesionales para el EMPLEADOR (user_type === 'empleador'), acotada a
// SU empresa por el backend (/users/employees). Tabla clásica, espejo del panel
// admin (Admin.tsx) pero dentro del tenant del empleador.
export default function EmpresaEmployees() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { user: employer } = useAuth()
  const [search, setSearch] = useState('')

  // Modal "Crear profesional".
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ name: '', email: '', job_title: '' })
  const [creating, setCreating] = useState(false)
  const [createErr, setCreateErr] = useState<string | null>(null)
  // Contraseña temporal devuelta por el backend (se muestra una sola vez).
  const [tempPassword, setTempPassword] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['employer', 'employees'],
    queryFn: () => userService.getEmployees(),
  })

  const openCreate = () => {
    setForm({ name: '', email: '', job_title: '' })
    setCreateErr(null)
    setTempPassword(null)
    setCopied(false)
    setShowCreate(true)
  }

  const closeCreate = () => {
    if (creating) return
    setShowCreate(false)
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setCreating(true)
    setCreateErr(null)
    try {
      const payload: { name: string; email: string; job_title?: string } = {
        name: form.name.trim(),
        email: form.email.trim(),
      }
      const jt = form.job_title.trim()
      if (jt) payload.job_title = jt
      const res = await employerService.createEmployee(payload)
      setTempPassword(res.temp_password)
      queryClient.invalidateQueries({ queryKey: ['employer', 'employees'] })
    } catch (err: any) {
      setCreateErr(err?.response?.data?.error ?? 'No se pudo crear el profesional.')
    } finally {
      setCreating(false)
    }
  }

  const copyTempPassword = async () => {
    if (!tempPassword) return
    try {
      await navigator.clipboard.writeText(tempPassword)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch { /* clipboard no disponible: el usuario la copia a mano */ }
  }

  const employees = useMemo(() => data ?? [], [data])
  const totalManagers = employees.filter((u) => u.is_manager).length
  const companyName = employer?.company_name || employer?.name || 'Mi empresa'

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return employees
    return employees.filter(
      (u) => u.name?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q),
    )
  }, [employees, search])

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.6rem', fontWeight: 700, color: '#0f172a' }}>Profesionales</h1>
          <p style={{ margin: '4px 0 1.25rem', color: '#64748b', fontSize: '0.9rem' }}>
            {companyName} · {employees.length} profesional(es) · {totalManagers} manager(s)
          </p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '0.6rem 1rem', borderRadius: 10, border: 'none', background: '#2563eb', color: '#fff', fontWeight: 600, fontSize: '0.9rem', cursor: 'pointer', flexShrink: 0 }}
        >
          <UserPlus size={16} /> Crear profesional
        </button>
      </div>

      {/* Buscador */}
      <div style={{ position: 'relative', marginBottom: '1.25rem', maxWidth: 420 }}>
        <Search size={18} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8' }} />
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Buscar por nombre o email..."
          style={{ width: '100%', padding: '0.65rem 0.75rem 0.65rem 2.4rem', borderRadius: 10, border: '1px solid #e2e8f0', fontSize: '0.95rem', outline: 'none', boxSizing: 'border-box' }}
        />
      </div>

      {isLoading ? (
        <div style={{ display: 'grid', gap: '0.5rem' }}>
          {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} height={48} />)}
        </div>
      ) : isError ? (
        <div style={{ padding: '2rem', textAlign: 'center', color: '#ef4444' }}>No se pudieron cargar los profesionales.</div>
      ) : (
        <div className={styles['users-table']}>
          <table>
            <thead>
              <tr>
                <th>Profesional</th>
                <th>Email</th>
                <th>Tipo</th>
                <th>Estado</th>
                <th>Acciones</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ textAlign: 'center', padding: '32px 16px', color: '#94a3b8' }}>
                    {search ? 'No hay profesionales que coincidan con la búsqueda.' : 'Aún no hay profesionales en tu empresa.'}
                  </td>
                </tr>
              )}
              {filtered.map((u) => (
                <tr key={u.id}>
                  <td>
                    <div className={styles['user-cell']}>
                      <Avatar src={u.avatar} name={u.name} size="sm" />
                      <span>{u.name}</span>
                    </div>
                  </td>
                  <td>{u.email}</td>
                  <td>
                    <span className={`${styles['badge']} ${styles[u.user_type] || ''}`}>
                      {u.is_manager ? 'Manager' : u.user_type}
                    </span>
                  </td>
                  <td>
                    <span className={`${styles['status-badge']} ${u.is_active ? styles['active'] : styles['inactive']}`}>
                      {u.is_active ? 'Activo' : 'Inactivo'}
                    </span>
                  </td>
                  <td>
                    <button
                      type="button"
                      onClick={() => navigate(`/empresa/employees/${u.id}`)}
                      title="Ver detalle"
                      style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '0.45rem 0.9rem', borderRadius: 8, border: '1px solid #e2e8f0', background: '#f8fafc', color: '#334155', fontWeight: 600, fontSize: '0.85rem', cursor: 'pointer' }}
                    >
                      <Eye size={15} /> Ver
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <Modal
          isOpen
          onClose={closeCreate}
          title="Crear profesional"
          size="md"
          footer={
            tempPassword ? (
              <Button onClick={() => setShowCreate(false)}>Listo</Button>
            ) : (
              <>
                <Button variant="secondary" onClick={closeCreate} disabled={creating}>Cancelar</Button>
                <Button type="submit" form="create-employee-form" loading={creating}>Crear profesional</Button>
              </>
            )
          }
        >
          {tempPassword ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
              <div style={{ padding: '0.7rem 0.9rem', borderRadius: 10, background: 'rgba(16,185,129,0.1)', color: '#059669', fontSize: '0.88rem', fontWeight: 600 }}>
                Profesional creado correctamente.
              </div>
              <div>
                <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Contraseña temporal</label>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <code style={{ flex: 1, padding: '0.6rem 0.75rem', borderRadius: 8, border: '1px solid #e2e8f0', background: '#f8fafc', fontSize: '0.95rem', fontFamily: 'monospace', userSelect: 'all', wordBreak: 'break-all' }}>{tempPassword}</code>
                  <Button type="button" variant="secondary" onClick={copyTempPassword}>
                    {copied ? <Check size={15} /> : <Copy size={15} />} {copied ? 'Copiada' : 'Copiar'}
                  </Button>
                </div>
              </div>
              <p style={{ margin: 0, fontSize: '0.8rem', color: '#b45309', fontWeight: 600 }}>
                Compártela por un canal seguro; no se vuelve a mostrar.
              </p>
            </div>
          ) : (
            <form id="create-employee-form" onSubmit={handleCreate} className={styles['user-form']}>
              <div className={styles['form-group']}>
                <label>Nombre</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={e => setForm({ ...form, name: e.target.value })}
                  required
                  autoFocus
                />
              </div>
              <div className={styles['form-group']}>
                <label>Email</label>
                <input
                  type="email"
                  value={form.email}
                  onChange={e => setForm({ ...form, email: e.target.value })}
                  required
                />
              </div>
              <div className={styles['form-group']}>
                <label>Cargo</label>
                <input
                  type="text"
                  value={form.job_title}
                  onChange={e => setForm({ ...form, job_title: e.target.value })}
                />
              </div>
              {createErr && (
                <div style={{ color: '#dc2626', fontWeight: 600, fontSize: '0.85rem' }}>{createErr}</div>
              )}
            </form>
          )}
        </Modal>
      )}
    </div>
  )
}
