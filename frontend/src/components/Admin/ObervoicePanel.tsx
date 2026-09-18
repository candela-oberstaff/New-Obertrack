/**
 * ObervoicePanel — pestaña "Obervoice" del panel de administración.
 *
 * Muestra la lista paginada de usuarios con sus datos de telefonía.
 * Al seleccionar uno o hacer clic en Editar se abre un Modal para modificar
 * las credenciales y enviárselas por correo.
 *
 * Features:
 *  - Formulario de edición de credenciales SIP
 *  - Carga de imagen QR por usuario (base64)
 *  - Vista previa del email antes de enviarlo
 *  - Envío del correo de credenciales
 */
import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import {
  Search, Save, Check, X, RefreshCw,
  User as UserIcon, Mail, Hash, PhoneCall,
  Edit2, ChevronLeft, ChevronRight, ExternalLink,
  Eye, Send, ImagePlus, Trash2,
} from 'lucide-react'
import { adminService, uploadService } from '../../services/api'
import { User } from '../../types'
import Avatar from '../Common/Avatar'
import { Modal } from '../ui'

// ── helpers ────────────────────────────────────────────────────────────────

function hasObervoice(u: User) {
  return !!(u.obervoice_username || u.obervoice_extension || (u as any).obervoice_phone)
}

// ── tipos locales ──────────────────────────────────────────────────────────

interface ObervoiceForm {
  obervoice_username: string
  obervoice_password: string
  obervoice_extension: string
  obervoice_prefix: string
  obervoice_phone: string
  obervoice_qr: string | null   // base64 data-URL o null para borrar
}

const EMPTY_FORM: ObervoiceForm = {
  obervoice_username: '',
  obervoice_password: '',
  obervoice_extension: '',
  obervoice_prefix: '',
  obervoice_phone: '',
  obervoice_qr: null,
}

// ── componente ────────────────────────────────────────────────────────────

export default function ObervoicePanel() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<User | null>(null)
  const [form, setForm] = useState<ObervoiceForm>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [sending, setSending] = useState(false)
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)

  // Vista previa
  const [previewHtml, setPreviewHtml] = useState<string | null>(null)
  const [loadingPreview, setLoadingPreview] = useState(false)

  // Paginación
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const qrInputRef = useRef<HTMLInputElement>(null)

  const loadUsers = useCallback(async () => {
    setLoading(true)
    try {
      const res: any = await adminService.getUsers({ limit: 1000 })
      const all: User[] = res?.data ?? (Array.isArray(res) ? res : [])
      setUsers(all.filter(u => u.is_active))
    } catch {
      /* silencioso */
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadUsers() }, [loadUsers])

  const selectUser = (u: User) => {
    setSelected(u)
    setMsg(null)
    setPreviewHtml(null)
    setForm({
      obervoice_username:  u.obervoice_username  || '',
      obervoice_password:  '',                        // nunca viene del backend
      obervoice_extension: u.obervoice_extension || '',
      obervoice_prefix:    (u as any).obervoice_prefix || '',
      obervoice_phone:     (u as any).obervoice_phone  || '',
      obervoice_qr:        (u as any).obervoice_qr     || null,
    })
  }

  const closeModal = () => {
    setSelected(null)
    setMsg(null)
    setPreviewHtml(null)
    setForm(EMPTY_FORM)
  }

  const handleSave = async () => {
    if (!selected) return
    setSaving(true); setMsg(null)
    try {
      await adminService.updateUser(selected.id, {
        obervoice_username:  form.obervoice_username  || undefined,
        obervoice_password:  form.obervoice_password  || undefined,
        obervoice_extension: form.obervoice_extension || undefined,
        obervoice_prefix:    form.obervoice_prefix    || undefined,
        obervoice_phone:     form.obervoice_phone     || undefined,
        // Enviamos null para borrar o string para guardar
        ...(form.obervoice_qr !== undefined && { obervoice_qr: form.obervoice_qr }),
      })
      setMsg({ ok: true, text: 'Datos guardados correctamente.' })
      setUsers(prev => prev.map(u => u.id === selected.id
        ? { ...u, ...form, obervoice_password: undefined } as any
        : u
      ))
      setSelected(prev => prev ? { ...prev, ...form } as any : null)
    } catch (e: any) {
      setMsg({ ok: false, text: e?.response?.data?.error || 'No se pudo guardar.' })
    } finally {
      setSaving(false)
    }
  }

  const handleSendCredentials = async () => {
    if (!selected) return
    setSending(true); setMsg(null)
    try {
      await fetch(`/api/admin/obervoice/send-credentials/${selected.id}`, {
        method: 'POST',
        credentials: 'include',
      }).then(async r => {
        if (!r.ok) {
          const d = await r.json().catch(() => ({}))
          throw new Error(d.error || `Error ${r.status}`)
        }
      })
      setMsg({ ok: true, text: `Credenciales enviadas a ${selected.email}.` })
    } catch (e: any) {
      setMsg({ ok: false, text: e?.message || 'No se pudo enviar el correo.' })
    } finally {
      setSending(false)
    }
  }

  const handlePreview = async () => {
    if (!selected) return
    setLoadingPreview(true); setMsg(null)
    try {
      const r = await fetch(`/api/admin/obervoice/preview-credentials/${selected.id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          name: selected.name,
          obervoice_username: form.obervoice_username,
          obervoice_password: form.obervoice_password,
          obervoice_extension: form.obervoice_extension,
          obervoice_prefix: form.obervoice_prefix,
          obervoice_phone: form.obervoice_phone,
          obervoice_qr: form.obervoice_qr,
        }),
      })
      if (!r.ok) {
        const d = await r.json().catch(() => ({}))
        throw new Error(d.error || `Error ${r.status}`)
      }
      const data = await r.json()
      setPreviewHtml(data.html || '')
    } catch (e: any) {
      setMsg({ ok: false, text: e?.message || 'No se pudo generar la vista previa.' })
    } finally {
      setLoadingPreview(false)
    }
  }

  const handleQrUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const res = await uploadService.upload(file)
      setForm(f => ({ ...f, obervoice_qr: res.url }))
    } catch {
      const reader = new FileReader()
      reader.onload = ev => {
        setForm(f => ({ ...f, obervoice_qr: ev.target?.result as string }))
      }
      reader.readAsDataURL(file)
    }
    e.target.value = ''
  }

  const handleRemoveQr = () => {
    setForm(f => ({ ...f, obervoice_qr: null }))
  }

  // ── search & pagination ─────────────────────────────────────────────────

  const handleSearchChange = (val: string) => {
    setSearch(val)
    setPage(1)
  }

  const filtered = useMemo(() => {
    const q = search.toLowerCase().trim()
    if (!q) return users
    return users.filter(u =>
      u.name.toLowerCase().includes(q) ||
      u.email.toLowerCase().includes(q) ||
      (u.company_name || '').toLowerCase().includes(q) ||
      (u.obervoice_extension || '').toLowerCase().includes(q) ||
      ((u as any).obervoice_phone || '').toLowerCase().includes(q)
    )
  }, [users, search])

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize))
  const safePage = Math.min(page, totalPages)

  const paginatedUsers = useMemo(() => {
    const start = (safePage - 1) * pageSize
    return filtered.slice(start, start + pageSize)
  }, [filtered, safePage, pageSize])

  // ── render ───────────────────────────────────────────────────────────────

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, paddingBottom: 60, marginBottom: 40 }}>

      {/* ── Barra de búsqueda y acciones ── */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <div style={{ position: 'relative', flex: 1, minWidth: 240 }}>
          <Search size={15} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8', pointerEvents: 'none' }} />
          <input
            value={search}
            onChange={e => handleSearchChange(e.target.value)}
            placeholder="Buscar por nombre, email, empresa, extensión o teléfono…"
            style={{ width: '100%', paddingLeft: 34, paddingRight: 12, paddingTop: 9, paddingBottom: 9, borderRadius: 10, border: '1px solid var(--glass-border,#e2e8f0)', fontSize: 14, boxSizing: 'border-box', background: '#fff' }}
          />
        </div>
        <button
          onClick={loadUsers}
          disabled={loading}
          style={{
            display: 'flex', alignItems: 'center', gap: 6, padding: '9px 16px', borderRadius: 10,
            border: '1px solid var(--glass-border,#e2e8f0)', background: '#fff', color: '#475569',
            fontWeight: 600, fontSize: 13, cursor: loading ? 'not-allowed' : 'pointer', whiteSpace: 'nowrap'
          }}
        >
          <RefreshCw size={14} style={{ animation: loading ? 'spin 1s linear infinite' : 'none' }} />
          {loading ? 'Cargando…' : 'Actualizar'}
        </button>
        <a
          href="https://voice.oberstaff.com/webrtc/login"
          target="_blank"
          rel="noopener noreferrer"
          style={{
            display: 'flex', alignItems: 'center', gap: 6, padding: '9px 16px', borderRadius: 10,
            border: '1px solid var(--primary,#cc33cc)', background: 'rgba(204,51,204,0.06)', color: 'var(--primary,#cc33cc)',
            fontWeight: 700, fontSize: 13, textDecoration: 'none', whiteSpace: 'nowrap', transition: 'all 0.15s'
          }}
        >
          <PhoneCall size={14} /> Obervoice <ExternalLink size={13} style={{ opacity: 0.8 }} />
        </a>
      </div>

      {/* ── Tabla de usuarios ── */}
      <div style={{ background: '#fff', border: '1px solid var(--glass-border,#e2e8f0)', borderRadius: 14, overflow: 'hidden', boxShadow: '0 1px 3px rgba(0,0,0,0.05)' }}>
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: '#f8fafc', borderBottom: '1px solid var(--glass-border,#e2e8f0)' }}>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 700, color: '#64748b' }}>Usuario</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 700, color: '#64748b' }}>Empresa</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 700, color: '#64748b' }}>Extensión</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 700, color: '#64748b' }}>Teléfono</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 700, color: '#64748b' }}>Estado</th>
                <th style={{ padding: '12px 16px', textAlign: 'right', fontWeight: 700, color: '#64748b' }}>Acciones</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ padding: '36px', textAlign: 'center', color: '#94a3b8' }}>
                    {loading ? 'Cargando usuarios…' : 'No se encontraron usuarios.'}
                  </td>
                </tr>
              )}
              {paginatedUsers.map(u => {
                const configured = hasObervoice(u)
                return (
                  <tr
                    key={u.id}
                    onClick={() => selectUser(u)}
                    style={{
                      borderBottom: '1px solid var(--glass-border,#f1f5f9)',
                      cursor: 'pointer',
                      transition: 'background 0.1s',
                    }}
                    onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = '#f8fafc' }}
                    onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'transparent' }}
                  >
                    <td style={{ padding: '10px 16px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <Avatar src={u.avatar} name={u.name} size="sm" />
                        <div>
                          <div style={{ fontWeight: 600, color: '#0f172a' }}>{u.name}</div>
                          <div style={{ fontSize: 11, color: '#94a3b8' }}>{u.email}</div>
                        </div>
                      </div>
                    </td>
                    <td style={{ padding: '10px 16px', color: '#64748b' }}>{u.company_name || '—'}</td>
                    <td style={{ padding: '10px 16px', fontFamily: 'monospace', color: '#0f172a', fontWeight: 600 }}>{u.obervoice_extension || '—'}</td>
                    <td style={{ padding: '10px 16px', color: '#0f172a' }}>{(u as any).obervoice_phone || '—'}</td>
                    <td style={{ padding: '10px 16px' }}>
                      <span style={{
                        fontSize: 11, fontWeight: 700, padding: '3px 9px', borderRadius: 99,
                        background: configured ? 'rgba(16,185,129,0.1)' : 'rgba(148,163,184,0.12)',
                        color: configured ? '#059669' : '#94a3b8',
                        display: 'inline-flex', alignItems: 'center', gap: 4
                      }}>
                        {configured ? '✓ Configurado' : 'Sin datos'}
                      </span>
                    </td>
                    <td style={{ padding: '10px 16px', textAlign: 'right' }}>
                      <button
                        type="button"
                        onClick={(e) => { e.stopPropagation(); selectUser(u) }}
                        style={{
                          display: 'inline-flex', alignItems: 'center', gap: 6,
                          padding: '6px 12px', borderRadius: 8, border: '1px solid #cbd5e1',
                          background: '#fff', color: 'var(--primary,#cc33cc)', fontWeight: 600,
                          fontSize: 12, cursor: 'pointer', transition: 'all 0.15s'
                        }}
                      >
                        <Edit2 size={13} />
                        Editar
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>

        {/* ── Controles de Paginación ── */}
        {filtered.length > 0 && (
          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '12px 16px', borderTop: '1px solid var(--glass-border,#e2e8f0)',
            background: '#f8fafc', flexWrap: 'wrap', gap: 12
          }}>
            <div style={{ fontSize: 13, color: '#64748b' }}>
              Mostrando <strong>{(safePage - 1) * pageSize + 1}</strong>–<strong>{Math.min(safePage * pageSize, filtered.length)}</strong> de <strong>{filtered.length}</strong> usuarios
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13, color: '#64748b' }}>
                <span>Por página:</span>
                <select
                  value={pageSize}
                  onChange={e => { setPageSize(Number(e.target.value)); setPage(1) }}
                  style={{
                    padding: '4px 8px', borderRadius: 6, border: '1px solid #cbd5e1',
                    fontSize: 13, background: '#fff', cursor: 'pointer', outline: 'none'
                  }}
                >
                  <option value={10}>10</option>
                  <option value={20}>20</option>
                  <option value={50}>50</option>
                </select>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <button
                  type="button"
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={safePage <= 1}
                  style={{
                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    padding: '6px 10px', borderRadius: 8, border: '1px solid #cbd5e1',
                    background: '#fff', color: '#334155', cursor: safePage <= 1 ? 'not-allowed' : 'pointer',
                    opacity: safePage <= 1 ? 0.4 : 1, fontSize: 13
                  }}
                  title="Página anterior"
                >
                  <ChevronLeft size={16} />
                </button>
                <span style={{ fontSize: 13, fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                  Página {safePage} de {totalPages}
                </span>
                <button
                  type="button"
                  onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                  disabled={safePage >= totalPages}
                  style={{
                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    padding: '6px 10px', borderRadius: 8, border: '1px solid #cbd5e1',
                    background: '#fff', color: '#334155', cursor: safePage >= totalPages ? 'not-allowed' : 'pointer',
                    opacity: safePage >= totalPages ? 0.4 : 1, fontSize: 13
                  }}
                  title="Página siguiente"
                >
                  <ChevronRight size={16} />
                </button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* ── Modal de edición de datos Obervoice ── */}
      <Modal
        isOpen={!!selected && !previewHtml}
        onClose={closeModal}
        size="lg"
        title={
          selected ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <Avatar src={selected.avatar} name={selected.name} size="md" />
              <div>
                <div style={{ fontWeight: 700, fontSize: 16, color: '#0f172a' }}>{selected.name}</div>
                <div style={{ fontSize: 12, color: '#64748b', fontWeight: 400 }}>{selected.email}</div>
              </div>
            </div>
          ) : undefined
        }
      >
        {selected && (
          <div>
            <p style={{ margin: '0 0 16px 0', fontSize: 13, color: '#64748b' }}>
              Configurá los datos de telefonía SIP para este usuario.
            </p>

            {/* Formulario */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <Field label="Usuario SIP" icon={<UserIcon size={13} />}>
                <input
                  value={form.obervoice_username}
                  onChange={e => setForm(f => ({ ...f, obervoice_username: e.target.value }))}
                  placeholder={selected.email}
                  style={inputStyle}
                />
              </Field>

              <Field label="Contraseña SIP" icon={<Hash size={13} />} hint="Dejar vacío para no cambiar">
                <input
                  type="password"
                  value={form.obervoice_password}
                  onChange={e => setForm(f => ({ ...f, obervoice_password: e.target.value }))}
                  autoComplete="new-password"
                  placeholder="••••••••"
                  style={inputStyle}
                />
              </Field>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                <Field label="Extensión" icon={<Hash size={13} />}>
                  <input
                    value={form.obervoice_extension}
                    onChange={e => setForm(f => ({ ...f, obervoice_extension: e.target.value }))}
                    placeholder="1001"
                    style={inputStyle}
                  />
                </Field>
                <Field label="Prefijo" icon={<Hash size={13} />}>
                  <input
                    value={form.obervoice_prefix}
                    onChange={e => setForm(f => ({ ...f, obervoice_prefix: e.target.value }))}
                    placeholder="9"
                    style={inputStyle}
                  />
                </Field>
              </div>

              <Field label="Teléfono asignado" icon={<PhoneCall size={13} />}>
                <input
                  value={form.obervoice_phone}
                  onChange={e => setForm(f => ({ ...f, obervoice_phone: e.target.value }))}
                  placeholder="+34 900 123 456"
                  style={inputStyle}
                />
              </Field>

              {/* QR Upload */}
              <Field label="QR de acceso (adjunto en el correo)" icon={<ImagePlus size={13} />} hint="Opcional">
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {form.obervoice_qr ? (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                      <img
                        src={form.obervoice_qr}
                        alt="QR preview"
                        style={{ width: 72, height: 72, objectFit: 'contain', borderRadius: 8, border: '1px solid #e2e8f0', background: '#f8fafc' }}
                      />
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                        <span style={{ fontSize: 12, color: '#059669', fontWeight: 600 }}>✓ QR cargado</span>
                        <button
                          type="button"
                          onClick={() => qrInputRef.current?.click()}
                          style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '5px 10px', borderRadius: 6, border: '1px solid #cbd5e1', background: '#fff', fontSize: 12, color: '#475569', cursor: 'pointer', fontWeight: 600 }}
                        >
                          <ImagePlus size={12} /> Cambiar imagen
                        </button>
                        <button
                          type="button"
                          onClick={handleRemoveQr}
                          style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '5px 10px', borderRadius: 6, border: '1px solid #fecaca', background: '#fff', fontSize: 12, color: '#dc2626', cursor: 'pointer', fontWeight: 600 }}
                        >
                          <Trash2 size={12} /> Eliminar QR
                        </button>
                      </div>
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={() => qrInputRef.current?.click()}
                      style={{
                        display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                        padding: '18px 12px', borderRadius: 10, border: '2px dashed #cbd5e1',
                        background: '#f8fafc', color: '#64748b', fontSize: 13, cursor: 'pointer',
                        fontWeight: 600, transition: 'all 0.15s', width: '100%', boxSizing: 'border-box',
                      }}
                    >
                      <ImagePlus size={16} /> Cargar imagen QR
                    </button>
                  )}
                  <input
                    ref={qrInputRef}
                    type="file"
                    accept="image/*"
                    style={{ display: 'none' }}
                    onChange={handleQrUpload}
                  />
                  <p style={{ margin: 0, fontSize: 11, color: '#94a3b8', lineHeight: 1.4 }}>
                    Se adjuntará como imagen en el correo. Formatos aceptados: PNG, JPG, GIF, WebP.
                  </p>
                </div>
              </Field>
            </div>

            {/* Mensaje de estado */}
            {msg && (
              <div style={{
                marginTop: 16, padding: '10px 14px', borderRadius: 8, fontSize: 13,
                background: msg.ok ? 'rgba(16,185,129,0.08)' : 'rgba(239,68,68,0.08)',
                border: `1px solid ${msg.ok ? 'rgba(16,185,129,0.2)' : 'rgba(239,68,68,0.2)'}`,
                color: msg.ok ? '#059669' : '#dc2626',
                display: 'flex', alignItems: 'center', gap: 8,
              }}>
                {msg.ok ? <Check size={15} /> : <X size={15} />}
                {msg.text}
              </div>
            )}

            {/* Acciones */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 22 }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>

                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  {/* Vista Previa */}
                  <button
                    type="button"
                    onClick={handlePreview}
                    disabled={loadingPreview || !hasObervoice(selected)}
                    title={!hasObervoice(selected) ? 'Guardá los datos primero' : 'Ver cómo quedará el correo antes de enviarlo'}
                    style={{
                      display: 'inline-flex', alignItems: 'center', gap: 6,
                      padding: '8px 12px', borderRadius: 10,
                      cursor: (loadingPreview || !hasObervoice(selected)) ? 'not-allowed' : 'pointer',
                      background: 'transparent', border: '1px solid #6366f1',
                      color: '#6366f1', fontWeight: 700, fontSize: 13,
                      opacity: (loadingPreview || !hasObervoice(selected)) ? 0.5 : 1,
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <Eye size={14} />
                    {loadingPreview ? 'Generando…' : 'Vista previa'}
                  </button>

                  {/* Enviar correo */}
                  <button
                    type="button"
                    onClick={handleSendCredentials}
                    disabled={sending || !hasObervoice(selected)}
                    title={!hasObervoice(selected) ? 'Guardá los datos primero' : 'Enviar credenciales por correo'}
                    style={{
                      display: 'inline-flex', alignItems: 'center', gap: 6,
                      padding: '8px 12px', borderRadius: 10, cursor: (sending || !hasObervoice(selected)) ? 'not-allowed' : 'pointer',
                      background: 'transparent', border: '1px solid var(--primary,#cc33cc)',
                      color: 'var(--primary,#cc33cc)', fontWeight: 700, fontSize: 13,
                      opacity: (sending || !hasObervoice(selected)) ? 0.5 : 1,
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <Mail size={14} />
                    {sending ? 'Enviando…' : 'Enviar por correo'}
                  </button>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <button
                    type="button"
                    onClick={closeModal}
                    style={{
                      padding: '8px 14px', borderRadius: 10, border: '1px solid #cbd5e1',
                      background: '#fff', color: '#475569', fontWeight: 600, fontSize: 13, cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    Cancelar
                  </button>

                  <button
                    type="button"
                    onClick={handleSave}
                    disabled={saving}
                    style={{
                      display: 'inline-flex', alignItems: 'center', gap: 6,
                      padding: '8px 14px', borderRadius: 10, border: 'none', cursor: saving ? 'not-allowed' : 'pointer',
                      background: 'var(--primary,#cc33cc)', color: '#fff', fontWeight: 700, fontSize: 13,
                      opacity: saving ? 0.7 : 1, whiteSpace: 'nowrap',
                    }}
                  >
                    <Save size={14} />
                    {saving ? 'Guardando…' : 'Guardar datos'}
                  </button>
                </div>

              </div>

              <p style={{ margin: '4px 0 0', fontSize: 11, color: '#94a3b8', lineHeight: 1.4 }}>
                El correo incluirá usuario, extensión, prefijo y teléfono. La contraseña solo se incluye si está guardada. Si hay un QR cargado, se adjuntará al correo.
              </p>
            </div>
          </div>
        )}
      </Modal>

      {/* ── Modal de Vista Previa del email ── */}
      <Modal
        isOpen={!!previewHtml}
        onClose={() => setPreviewHtml(null)}
        size="lg"
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <Eye size={18} style={{ color: '#6366f1' }} />
            <span style={{ fontWeight: 700, fontSize: 15, color: '#0f172a' }}>
              Vista previa del correo — {selected?.name}
            </span>
          </div>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{
            padding: '8px 12px', background: 'rgba(99,102,241,0.06)', borderRadius: 8,
            border: '1px solid rgba(99,102,241,0.15)', fontSize: 12, color: '#6366f1', fontWeight: 600,
            display: 'flex', alignItems: 'center', gap: 6
          }}>
            <Eye size={13} />
            Esta es una vista previa del email que recibirá el usuario. Revisá el contenido antes de enviarlo.
          </div>

          {/* Iframe para renderizar el HTML del email */}
          <div style={{
            border: '1px solid #e2e8f0', borderRadius: 12, overflow: 'hidden',
            boxShadow: '0 2px 8px rgba(0,0,0,0.06)'
          }}>
            <iframe
              srcDoc={previewHtml ?? ''}
              style={{ width: '100%', height: 560, border: 'none', display: 'block' }}
              title="Vista previa del email"
              sandbox="allow-same-origin"
            />
          </div>

          {/* Mensaje de estado en preview */}
          {msg && (
            <div style={{
              padding: '10px 14px', borderRadius: 8, fontSize: 13,
              background: msg.ok ? 'rgba(16,185,129,0.08)' : 'rgba(239,68,68,0.08)',
              border: `1px solid ${msg.ok ? 'rgba(16,185,129,0.2)' : 'rgba(239,68,68,0.2)'}`,
              color: msg.ok ? '#059669' : '#dc2626',
              display: 'flex', alignItems: 'center', gap: 8,
            }}>
              {msg.ok ? <Check size={15} /> : <X size={15} />}
              {msg.text}
            </div>
          )}

          <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
            <button
              type="button"
              onClick={() => setPreviewHtml(null)}
              style={{
                padding: '9px 16px', borderRadius: 10, border: '1px solid #cbd5e1',
                background: '#fff', color: '#475569', fontWeight: 600, fontSize: 13, cursor: 'pointer'
              }}
            >
              Volver a editar
            </button>
            <button
              type="button"
              onClick={async () => {
                await handleSendCredentials()
                setPreviewHtml(null)
              }}
              disabled={sending}
              style={{
                display: 'flex', alignItems: 'center', gap: 6,
                padding: '9px 20px', borderRadius: 10, border: 'none',
                background: 'var(--primary,#cc33cc)', color: '#fff',
                fontWeight: 700, fontSize: 13, cursor: sending ? 'not-allowed' : 'pointer',
                opacity: sending ? 0.7 : 1,
              }}
            >
              <Send size={14} />
              {sending ? 'Enviando…' : 'Confirmar y enviar'}
            </button>
          </div>
        </div>
      </Modal>

    </div>
  )
}

// ── sub-componentes ───────────────────────────────────────────────────────

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  borderRadius: 8,
  border: '1px solid var(--glass-border,#e2e8f0)',
  fontSize: 13,
  fontFamily: 'inherit',
  boxSizing: 'border-box',
}

function Field({ label, icon, hint, children }: {
  label: string
  icon?: React.ReactNode
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div>
      <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11, fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: 5 }}>
        {icon} {label}
        {hint && <span style={{ fontWeight: 400, textTransform: 'none', color: '#94a3b8', marginLeft: 4 }}>({hint})</span>}
      </label>
      {children}
    </div>
  )
}
