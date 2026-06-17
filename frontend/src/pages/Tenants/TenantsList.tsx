import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Building2, Users, Search, Plus, Ban, CheckCircle2, ChevronLeft, ChevronRight, X, Mail, MessageSquare } from 'lucide-react'
import { useTenants } from '../../hooks'
import { adminService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import type { Tenant, User } from '../../types'
import Avatar from '../../components/Common/Avatar'
import { Modal, Button, Skeleton } from '../../components/ui'
import { Select } from '../../components/ui/Select'
import { emailService } from '../../services/emailService'
import { EmailComposerModal, type ComposerRecipient } from '../../components/Admin/EmailComposerModal'
import styles from './Tenants.module.css'

export default function TenantsList() {
  const navigate = useNavigate()
  const { user: viewer } = useAuth()
  // CS entra en modo consulta: sin crear ni suspender empresas.
  const canManage = !!viewer?.is_superadmin
  const { tenants, isLoading, error, createTenant, suspendTenant, activateTenant } = useTenants()

  const [search, setSearch] = useState('')
  const [industryFilter, setIndustryFilter] = useState('')
  const [countryFilter, setCountryFilter] = useState('')
  const [page, setPage] = useState(1)
  const [showCreate, setShowCreate] = useState(false)
  const [companyName, setCompanyName] = useState('')
  const [responsableQuery, setResponsableQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [users, setUsers] = useState<User[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  // Estados para acciones de comunicación y selección masiva
  const [selectedTenantIds, setSelectedTenantIds] = useState<number[]>([])
  // Modal de redacción de correo (plantillas de Tools / redacción nueva) para una empresa.
  const [composer, setComposer] = useState<ComposerRecipient | null>(null)
  const [commModal, setCommModal] = useState<{
    isOpen: boolean
    type: 'email' | 'whatsapp'
    target: Tenant | 'bulk'
    subject: string
    message: string
  }>({
    isOpen: false,
    type: 'email',
    target: 'bulk',
    subject: '',
    message: '',
  })

  // Opciones de filtro derivadas de los datos cargados (solo valores presentes).
  const industries = Array.from(new Set(tenants.map(t => t.industry?.trim()).filter(Boolean))).sort((a, b) => a.localeCompare(b, 'es', { sensitivity: 'base' }))
  const countries = Array.from(new Set(tenants.map(t => t.country?.trim()).filter(Boolean))).sort((a, b) => a.localeCompare(b, 'es', { sensitivity: 'base' }))

  const filtered = tenants
    .filter(t => {
      const q = search.trim().toLowerCase()
      if (q && !(t.company_name?.toLowerCase().includes(q) || t.owner_email?.toLowerCase().includes(q))) return false
      if (industryFilter && t.industry?.trim() !== industryFilter) return false
      if (countryFilter && t.country?.trim() !== countryFilter) return false
      return true
    })
    .sort((a, b) => (a.company_name || '').localeCompare(b.company_name || '', 'es', { sensitivity: 'base' }))

  const TENANTS_PER_PAGE = 10
  const totalPages = Math.max(1, Math.ceil(filtered.length / TENANTS_PER_PAGE))
  const currentPage = Math.min(page, totalPages)
  const paginated = filtered.slice((currentPage - 1) * TENANTS_PER_PAGE, currentPage * TENANTS_PER_PAGE)

  useEffect(() => {
    setPage(1)
  }, [search, industryFilter, countryFilter])

  const hasFilters = !!(search.trim() || industryFilter || countryFilter)
  const clearFilters = () => {
    setSearch('')
    setIndustryFilter('')
    setCountryFilter('')
  }

  const activeCount = tenants.filter(t => t.is_active).length
  const totalUsers = tenants.reduce((sum, t) => sum + (t.user_count || 0), 0)

  // Server-side search for the "responsable" picker: query as the user types
  // (debounced) instead of downloading every user upfront.
  useEffect(() => {
    if (!showCreate) { setUsers([]); return }
    const term = responsableQuery.trim()
    if (term.length < 2) { setUsers([]); return }
    let active = true
    const t = setTimeout(() => {
      adminService.getUsers({ q: term, limit: 10 })
        .then(res => {
          const arr = res?.data || (Array.isArray(res) ? res : [])
          if (active) setUsers(arr)
        })
        .catch(() => {})
    }, 250)
    return () => { active = false; clearTimeout(t) }
  }, [showCreate, responsableQuery])

  const suggestions = responsableQuery.trim().length < 2
    ? []
    : users.filter(u =>
        !u.is_superadmin &&
        u.user_type !== 'empleador'
      ).slice(0, 6)

  const closeCreate = () => {
    setShowCreate(false)
    setCompanyName('')
    setResponsableQuery('')
    setSelectedUser(null)
    setFormError(null)
  }

  const handleCreate = async () => {
    if (!selectedUser) return
    setSubmitting(true)
    setFormError(null)
    try {
      await createTenant({ company_name: companyName, user_id: selectedUser.id })
      closeCreate()
    } catch (err: any) {
      setFormError(err?.response?.data?.error || 'No se pudo crear la empresa')
    } finally {
      setSubmitting(false)
    }
  }

  const handleToggle = async (e: React.MouseEvent, t: Tenant) => {
    e.stopPropagation()
    if (t.is_active) {
      await suspendTenant(t.id)
    } else {
      await activateTenant(t.id)
    }
  }

  // Manejadores para selección masiva
  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedTenantIds(paginated.map(t => t.id))
    } else {
      setSelectedTenantIds([])
    }
  }

  const handleSelectOne = (e: React.ChangeEvent<HTMLInputElement>, id: number) => {
    e.stopPropagation()
    if (e.target.checked) {
      setSelectedTenantIds([...selectedTenantIds, id])
    } else {
      setSelectedTenantIds(selectedTenantIds.filter(tid => tid !== id))
    }
  }

  const openCommModal = (e: React.MouseEvent, type: 'email' | 'whatsapp', target: Tenant | 'bulk') => {
    e.stopPropagation()
    setCommModal({
      isOpen: true,
      type,
      target,
      subject: type === 'email' ? 'Contacto desde Oberstaff' : '',
      message: ''
    })
  }

  const handleSendComm = async () => {
    const isBulk = commModal.target === 'bulk'
    
    if (commModal.type === 'email') {
      try {
        if (isBulk) {
          const targets = tenants.filter(t => selectedTenantIds.includes(t.id))
          const recipients = targets
            .map(t => ({ name: t.owner_name || t.company_name || '', email: t.owner_email || '' }))
            .filter(r => r.email)
          
          if (recipients.length === 0) {
            alert('Ninguna de las empresas seleccionadas tiene un correo electrónico válido.')
            return
          }

          await emailService.sendQuickEmailBulk({
            recipients,
            subject: commModal.subject || 'Contacto masivo Oberstaff',
            html_content: `<p>${commModal.message.replace(/\n/g, '<br>')}</p>`
          })
          alert(`¡Correo masivo enviado con éxito a ${recipients.length} destinatarios!`)
        } else {
          const target = commModal.target as Tenant
          if (!target.owner_email) {
            alert('Esta empresa no tiene un correo de contacto asociado.')
            return
          }
          await emailService.sendQuickEmail({
            to_email: target.owner_email,
            to_name: target.owner_name || target.company_name,
            subject: commModal.subject || 'Contacto oficial Oberstaff',
            html_content: `<p>${commModal.message.replace(/\n/g, '<br>')}</p>`
          })
          alert(`¡Correo enviado con éxito a ${target.owner_email}!`)
        }
      } catch (err) {
        console.error(err)
        alert('Error al enviar el correo. Por favor, intente de nuevo.')
      }
    } else {
      const targetNames = isBulk
        ? `${selectedTenantIds.length} empresas seleccionadas`
        : (commModal.target as Tenant).company_name

      console.log(`[SIMULACIÓN] Enviando ${commModal.type.toUpperCase()} a ${targetNames}:`, {
        asunto: commModal.subject,
        mensaje: commModal.message
      })
      alert(`¡Mensaje (${commModal.type.toUpperCase()}) enviado con éxito a ${targetNames}! (Simulado)`)
    }

    setCommModal(prev => ({ ...prev, isOpen: false, message: '', subject: '' }))
    if (isBulk) {
      setSelectedTenantIds([])
    }
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.header}>
          <div>
            <h1>Empresas</h1>
            <p>Gestiona los clientes de la plataforma Oberstaff</p>
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem', marginTop: '1rem' }}>
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} height={56} radius={12} />
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className={styles.page}>
      <div className={styles.header} data-tour="tenants-header">
        <div>
          <h1>Empresas</h1>
          <p>Gestiona los clientes de la plataforma Oberstaff</p>
        </div>
        {canManage && (
          <Button onClick={() => setShowCreate(true)} leftIcon={<Plus size={18} />} data-tour="tenants-create">
            Nueva empresa
          </Button>
        )}
      </div>

      <div className={styles.kpis} data-tour="tenants-kpis">
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: '#faf5ff', color: 'var(--primary)' }}>
            <Building2 size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{tenants.length}</span>
            <span className={styles.kpiLabel}>Empresas</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: '#ecfdf5', color: '#10b981' }}>
            <CheckCircle2 size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{activeCount}</span>
            <span className={styles.kpiLabel}>Activas</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: '#fef2f2', color: '#ef4444' }}>
            <Ban size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{tenants.length - activeCount}</span>
            <span className={styles.kpiLabel}>Suspendidas</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: '#f5f3ff', color: '#8b5cf6' }}>
            <Users size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{totalUsers}</span>
            <span className={styles.kpiLabel}>Profesionales totales</span>
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', marginBottom: 20 }}>
        <div className={styles.searchBox} style={{ margin: 0 }} data-tour="tenants-search">
          <Search size={18} />
          <input
            type="text"
            placeholder="Buscar empresa o correo..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <div style={{ minWidth: 190 }} data-tour="tenants-industry-filter">
          <Select
            fullWidth
            clearable
            placeholder="Todos los rubros"
            value={industryFilter}
            onChange={v => setIndustryFilter(v ? String(v) : '')}
            options={industries.map(i => ({ value: i, label: i }))}
          />
        </div>
        <div style={{ minWidth: 190 }} data-tour="tenants-country-filter">
          <Select
            fullWidth
            clearable
            placeholder="Todos los países"
            value={countryFilter}
            onChange={v => setCountryFilter(v ? String(v) : '')}
            options={countries.map(c => ({ value: c, label: c }))}
          />
        </div>
        {hasFilters && (
          <button
            type="button"
            onClick={clearFilters}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '9px 14px', border: '1px solid var(--glass-border)', borderRadius: '10px', background: 'transparent', color: '#64748b', fontSize: '13px', fontWeight: 600, cursor: 'pointer', whiteSpace: 'nowrap' }}
            title="Quitar todos los filtros"
          >
            <X size={14} /> Limpiar filtros
          </button>
        )}

        {/* Acciones Masivas cuando hay empresas seleccionadas */}
        {selectedTenantIds.length > 0 && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginLeft: 'auto', background: 'rgba(204, 51, 204, 0.08)', padding: '6px 12px', borderRadius: '10px', border: '1px dashed var(--primary)' }}>
            <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--primary)' }}>
              {selectedTenantIds.length} selec.
            </span>
            <Button
              size="sm"
              variant="secondary"
              leftIcon={<Mail size={14} />}
              onClick={(e) => openCommModal(e, 'email', 'bulk')}
            >
              Email Masivo
            </Button>
            <Button
              size="sm"
              variant="secondary"
              leftIcon={<MessageSquare size={14} />}
              onClick={(e) => openCommModal(e, 'whatsapp', 'bulk')}
            >
              WhatsApp Masivo
            </Button>
          </div>
        )}
      </div>

      {error && <p className={styles.errorMsg}>{error}</p>}

      {filtered.length === 0 ? (
        <div className={styles.empty} data-tour="tenants-list">
          <Building2 size={40} />
          <p>No hay empresas que coincidan</p>
        </div>
      ) : (
        <div className={styles.tableWrap} data-tour="tenants-list">
          <table className={styles.table}>
            <thead>
              <tr>
                <th style={{ width: '40px', paddingRight: 0 }}>
                  <input
                    type="checkbox"
                    checked={paginated.length > 0 && paginated.every(t => selectedTenantIds.includes(t.id))}
                    onChange={(e) => handleSelectAll(e.target.checked)}
                  />
                </th>
                <th>Empresa</th>
                <th>Responsable</th>
                <th>Profesionales</th>
                <th>Tableros</th>
                <th>Tareas</th>
                <th>Estado</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {paginated.map(t => (
                <tr key={t.id} className={styles.row} onClick={() => navigate(`/admin/tenants/${t.id}`)}>
                  <td style={{ paddingRight: 0 }} onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      checked={selectedTenantIds.includes(t.id)}
                      onChange={(e) => handleSelectOne(e, t.id)}
                    />
                  </td>
                  <td>
                    <div className={styles.companyCell}>
                      <div className={styles.companyLogo}>{t.company_name?.charAt(0).toUpperCase() || '?'}</div>
                      <span>{t.company_name}</span>
                    </div>
                  </td>
                  <td>
                    <div className={styles.ownerCell}>
                      <span>{t.owner_name}</span>
                      <small>{t.owner_email}</small>
                    </div>
                  </td>
                  <td>{t.user_count}</td>
                  <td>{t.board_count}</td>
                  <td>{t.task_count}</td>
                  <td>
                    <span className={`${styles.badge} ${t.is_active ? styles.badgeActive : styles.badgeSuspended}`}>
                      {t.is_active ? 'Activa' : 'Suspendida'}
                    </span>
                  </td>
                  <td>
                    <div className={styles.rowActions} onClick={(e) => e.stopPropagation()}>
                      <button
                        className={styles.iconBtn}
                        style={{ color: 'var(--primary)' }}
                        onClick={(e) => {
                          e.stopPropagation()
                          if (!t.owner_email) {
                            alert('Esta empresa no tiene un correo de contacto asociado.')
                            return
                          }
                          setComposer({ id: t.id, name: t.owner_name || t.company_name, email: t.owner_email })
                        }}
                        title="Enviar Correo"
                      >
                        <Mail size={16} />
                      </button>
                      <button
                        className={styles.iconBtn}
                        style={{ color: '#25D366' }}
                        onClick={(e) => openCommModal(e, 'whatsapp', t)}
                        title="Enviar WhatsApp"
                      >
                        <MessageSquare size={16} />
                      </button>
                      {canManage && (
                        <button
                          className={`${styles.iconBtn} ${t.is_active ? styles.danger : styles.success}`}
                          onClick={(e) => handleToggle(e, t)}
                          title={t.is_active ? 'Suspender empresa' : 'Reactivar empresa'}
                        >
                          {t.is_active ? <Ban size={16} /> : <CheckCircle2 size={16} />}
                        </button>
                      )}
                      <ChevronRight size={18} className={styles.chevron} onClick={() => navigate(`/admin/tenants/${t.id}`)} />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {filtered.length > 0 && (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 16px' }} data-tour="tenants-pagination">
              <span style={{ fontSize: '13px', color: '#64748b' }}>
                Mostrando {(currentPage - 1) * TENANTS_PER_PAGE + 1}–{Math.min(currentPage * TENANTS_PER_PAGE, filtered.length)} de {filtered.length} empresas
              </span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <button
                  type="button"
                  className={styles.iconBtn}
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={currentPage <= 1}
                  style={{ opacity: currentPage <= 1 ? 0.4 : 1, cursor: currentPage <= 1 ? 'not-allowed' : 'pointer' }}
                  title="Página anterior"
                >
                  <ChevronLeft size={16} />
                </button>
                <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                  Página {currentPage} de {totalPages}
                </span>
                <button
                  type="button"
                  className={styles.iconBtn}
                  onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                  disabled={currentPage >= totalPages}
                  style={{ opacity: currentPage >= totalPages ? 0.4 : 1, cursor: currentPage >= totalPages ? 'not-allowed' : 'pointer' }}
                  title="Página siguiente"
                >
                  <ChevronRight size={16} />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Modal de simulación de comunicación */}
      <Modal
        isOpen={commModal.isOpen}
        onClose={() => setCommModal(prev => ({ ...prev, isOpen: false }))}
        title={commModal.type === 'email' ? 'Enviar Correo (Simulación)' : 'Enviar WhatsApp (Simulación)'}
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setCommModal(prev => ({ ...prev, isOpen: false }))}>Cancelar</Button>
            <Button onClick={handleSendComm} disabled={!commModal.message}>
              {commModal.type === 'email' ? 'Enviar Email' : 'Enviar WhatsApp'}
            </Button>
          </>
        }
      >
        <p className={styles.modalHint}>
          {commModal.target === 'bulk'
            ? `Escribe el mensaje para las ${selectedTenantIds.length} empresas seleccionadas.`
            : `Escribe el mensaje para ${(commModal.target as Tenant).company_name}.`
          }
        </p>
        {commModal.type === 'email' && (
          <div className={styles.field}>
            <label>Asunto</label>
            <input
              type="text"
              value={commModal.subject}
              onChange={(e) => setCommModal(prev => ({ ...prev, subject: e.target.value }))}
              placeholder="Asunto del correo"
            />
          </div>
        )}
        <div className={styles.field}>
          <label>Mensaje</label>
          <textarea
            value={commModal.message}
            onChange={(e) => setCommModal(prev => ({ ...prev, message: e.target.value }))}
            placeholder={commModal.type === 'email' ? 'Escribe el cuerpo del correo aquí...' : 'Escribe tu mensaje de WhatsApp...'}
            rows={5}
            style={{ width: '100%', padding: '10px 14px', border: '1px solid #e2e8f0', borderRadius: 'var(--radius)', fontSize: '14px', outline: 'none' }}
          />
        </div>
      </Modal>

      <EmailComposerModal
        isOpen={composer !== null}
        onClose={() => setComposer(null)}
        recipient={composer}
        defaultSubject="Contacto oficial Oberstaff"
        logContact={false}
      />

      <Modal
        isOpen={showCreate}
        onClose={closeCreate}
        title="Nueva empresa"
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={closeCreate} disabled={submitting}>Cancelar</Button>
            <Button onClick={handleCreate} loading={submitting} disabled={!companyName || !selectedUser}>
              Crear empresa
            </Button>
          </>
        }
      >
        <p className={styles.modalHint}>Selecciona un usuario de la plataforma como responsable de la empresa.</p>
        <div className={styles.field}>
          <label>Nombre de la empresa</label>
          <input value={companyName} onChange={(e) => setCompanyName(e.target.value)} placeholder="Acme S.A." />
        </div>
        <div className={styles.field}>
          <label>Responsable</label>
          {selectedUser ? (
            <div className={styles.selectedChip}>
              <Avatar src={selectedUser.avatar} name={selectedUser.name} size="sm" />
              <div className={styles.ownerCell}>
                <span>{selectedUser.name}</span>
                <small>{selectedUser.email}</small>
              </div>
              <button className={styles.chipClear} onClick={() => { setSelectedUser(null); setResponsableQuery('') }}><X size={16} /></button>
            </div>
          ) : (
            <div className={styles.suggestWrap}>
              <div className={styles.searchBox} style={{ margin: 0, maxWidth: 'none' }}>
                <Search size={18} />
                <input
                  type="text"
                  value={responsableQuery}
                  onChange={(e) => setResponsableQuery(e.target.value)}
                  placeholder="Busca por nombre o correo..."
                />
              </div>
              {suggestions.length > 0 && (
                <div className={styles.suggestBox}>
                  {suggestions.map(u => (
                    <button key={u.id} className={styles.suggestItem} onClick={() => { setSelectedUser(u); setResponsableQuery('') }}>
                      <Avatar src={u.avatar} name={u.name} size="sm" />
                      <div className={styles.ownerCell}>
                        <span>{u.name}</span>
                        <small>{u.email} · {u.is_manager ? 'manager' : u.user_type}</small>
                      </div>
                    </button>
                  ))}
                </div>
              )}
              {responsableQuery.trim().length >= 2 && suggestions.length === 0 && (
                <p className={styles.suggestEmpty}>Sin usuarios que coincidan (se excluyen superadmins y empresas).</p>
              )}
            </div>
          )}
        </div>
        {formError && <p className={styles.errorMsg}>{formError}</p>}
      </Modal>
    </div>
  )
}

