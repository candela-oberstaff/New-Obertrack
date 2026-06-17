import { useState, useEffect, useCallback } from 'react'
import {
  Mail, Plus, Send, Trash2, X, Edit3, LayoutTemplate,
  CheckCircle, Clock, AlertCircle
} from 'lucide-react'
import { emailService, EmailCampaign, EmailTemplate } from '../../services/emailService'
import EmailBuilder, { EmailBlock, blocksToJSON } from './EmailBuilder'
import RecipientSelector, { RecipientValue } from './RecipientSelector'
import styles from './EmailCampaigns.module.css'

// ─── Toast ─────────────────────────────────────────────────────────────────
type ToastType = 'success' | 'error'
function Toast({ msg, type, onClose }: { msg: string; type: ToastType; onClose: () => void }) {
  useEffect(() => { const t = setTimeout(onClose, 4000); return () => clearTimeout(t) }, [onClose])
  return (
    <div className={`${styles.toast} ${type === 'success' ? styles.toastSuccess : styles.toastError}`}>
      {msg}
    </div>
  )
}

// ─── Campaign status badge ─────────────────────────────────────────────────
function StatusBadge({ status }: { status: string }) {
  const cls = status === 'sent' ? styles.badgeSent : status === 'scheduled' ? styles.badgeScheduled : styles.badgeDraft
  const label = status === 'sent' ? 'Enviada' : status === 'scheduled' ? 'Programada' : 'Borrador'
  const Icon = status === 'sent' ? CheckCircle : status === 'scheduled' ? Clock : Edit3
  return <span className={`${styles.badge} ${cls}`}><Icon size={10} />{label}</span>
}

// ─── Template editor modal ─────────────────────────────────────────────────
type TemplateStep = 'meta' | 'builder'

interface TemplateModalProps {
  initial?: EmailTemplate
  onSave: (t: Partial<EmailTemplate>) => Promise<void>
  onClose: () => void
  saving: boolean
}

function TemplateModal({ initial, onSave, onClose, saving }: TemplateModalProps) {
  const [step, setStep] = useState<TemplateStep>('meta')
  const [title, setTitle] = useState(initial?.title ?? '')
  const [subject, setSubject] = useState(initial?.subject ?? '')
  const [type, setType] = useState(initial?.type ?? 'campaign')
  const [blocks, setBlocks] = useState<EmailBlock[]>(() => {
    try {
      const raw = JSON.parse(initial?.content ?? '[]') as Array<{ type: string; content: string; style: Record<string, string> }>
      return raw.map((b, i) => ({ ...b, id: String(i), type: b.type as EmailBlock['type'] }))
    } catch { return [] }
  })

  const handleSave = async () => {
    await onSave({ title, subject, type, content: blocksToJSON(blocks), is_active: true })
  }

  return (
    <div className={styles.overlay}>
      <div className={styles.modal}>
        <div className={styles.modalHeader}>
          <h2 className={styles.modalTitle}>{initial ? 'Editar Plantilla' : 'Nueva Plantilla'}</h2>
          <button className={styles.modalClose} onClick={onClose}><X size={16} /></button>
        </div>
        <div className={styles.steps}>
          <div className={`${styles.step} ${step === 'meta' ? styles.activeStep : styles.doneStep}`}>1. Datos</div>
          <div className={`${styles.step} ${step === 'builder' ? styles.activeStep : ''}`}>2. Diseño</div>
        </div>
        <div className={styles.modalBody}>
          {step === 'meta' ? (
            <>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Nombre de la plantilla *</label>
                <input className={styles.formInput} value={title} onChange={e => setTitle(e.target.value)} placeholder="Ej: Newsletter Mensual" />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Asunto del email *</label>
                <input className={styles.formInput} value={subject} onChange={e => setSubject(e.target.value)} placeholder="Ej: ¡Novedades de este mes!" />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Tipo</label>
                <select className={styles.formSelect} value={type} onChange={e => setType(e.target.value)}>
                  <option value="campaign">Campaña</option>
                  <option value="transactional">Transaccional</option>
                </select>
              </div>
            </>
          ) : (
            <EmailBuilder blocks={blocks} onChange={setBlocks} />
          )}
        </div>
        <div className={styles.modalFooter}>
          {step === 'meta'
            ? <>
                <button className={styles.btnSecondary} onClick={onClose}>Cancelar</button>
                <button className={styles.btnPrimary} disabled={!title || !subject} onClick={() => setStep('builder')}>
                  Siguiente → Diseñar
                </button>
              </>
            : <>
                <button className={styles.btnSecondary} onClick={() => setStep('meta')}>← Atrás</button>
                <button className={styles.btnPrimary} disabled={saving || blocks.length === 0} onClick={handleSave}>
                  {saving ? 'Guardando...' : initial ? 'Guardar cambios' : 'Guardar plantilla'}
                </button>
              </>
          }
        </div>
      </div>
    </div>
  )
}

// ─── Campaign send modal ──────────────────────────────────────────────────
interface CampaignModalProps {
  initial?: EmailCampaign
  templates: EmailTemplate[]
  onSave: (c: Partial<EmailCampaign>) => Promise<EmailCampaign>
  onSend: (id: number) => Promise<void>
  onClose: () => void
  saving: boolean
}

type CampaignStep = 'setup' | 'recipients' | 'confirm'

function CampaignModal({ initial, templates, onSave, onSend, onClose, saving }: CampaignModalProps) {
  const [step, setStep] = useState<CampaignStep>('setup')
  const [title, setTitle] = useState(initial?.title ?? '')
  const [subject, setSubject] = useState(initial?.subject ?? '')
  const [templateId, setTemplateId] = useState<number | ''>(initial?.template_id ?? '')
  const [recipients, setRecipients] = useState<RecipientValue>({ userIds: [], groupIds: [], expressContacts: [] })
  const [savedCampaign, setSavedCampaign] = useState<EmailCampaign | null>(initial ?? null)

  const totalRecipients = recipients.userIds.length + (recipients.groupIds?.length ?? 0) + (recipients.expressContacts?.length ?? 0)

  const handleSaveDraft = async () => {
    const recipientList = JSON.stringify({
      userIds: recipients.userIds,
      groupIds: recipients.groupIds,
      expressContacts: recipients.expressContacts,
    })
    const saved = await onSave({ title, subject, template_id: templateId as number, status: 'draft', recipient_list: recipientList })
    setSavedCampaign(saved)
    return saved
  }

  const handleSend = async () => {
    if (!savedCampaign?.id) return
    await onSend(savedCampaign.id)
    onClose()
  }

  const selectedTemplate = templates.find(t => t.id === templateId)

  return (
    <div className={styles.overlay}>
      <div className={styles.modal} style={{ maxWidth: 680 }}>
        <div className={styles.modalHeader}>
          <h2 className={styles.modalTitle}>{initial ? 'Editar Campaña' : 'Nueva Campaña'}</h2>
          <button className={styles.modalClose} onClick={onClose}><X size={16} /></button>
        </div>
        <div className={styles.steps}>
          <div className={`${styles.step} ${step === 'setup' ? styles.activeStep : styles.doneStep}`}>1. Configurar</div>
          <div className={`${styles.step} ${step === 'recipients' ? styles.activeStep : step === 'confirm' ? styles.doneStep : ''}`}>2. Destinatarios</div>
          <div className={`${styles.step} ${step === 'confirm' ? styles.activeStep : ''}`}>3. Enviar</div>
        </div>
        <div className={styles.modalBody}>
          {step === 'setup' && (
            <>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Título de la campaña *</label>
                <input className={styles.formInput} value={title} onChange={e => setTitle(e.target.value)} placeholder="Ej: Newsletter Junio 2025" />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Asunto del email *</label>
                <input className={styles.formInput} value={subject} onChange={e => setSubject(e.target.value)} placeholder="Ej: ¡Novedades de este mes!" />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Plantilla *</label>
                <select className={styles.formSelect} value={templateId} onChange={e => setTemplateId(Number(e.target.value))}>
                  <option value="">— Seleccionar plantilla —</option>
                  {templates.map(t => <option key={t.id} value={t.id}>{t.title}</option>)}
                </select>
                {!templates.length && (
                  <p style={{ fontSize: 12, color: '#f59e0b', marginTop: 6 }}>
                    <AlertCircle size={12} style={{ verticalAlign: 'middle', marginRight: 4 }} />
                    No hay plantillas. Crea una en la pestaña "Plantillas" primero.
                  </p>
                )}
                {selectedTemplate && (
                  <div style={{ marginTop: 8, padding: '8px 12px', background: '#f5f3ff', borderRadius: 8, fontSize: 12, color: '#6d28d9' }}>
                    <strong>Asunto de la plantilla:</strong> {selectedTemplate.subject}
                  </div>
                )}
              </div>
            </>
          )}

          {step === 'recipients' && (
            <RecipientSelector value={recipients} onChange={setRecipients} />
          )}

          {step === 'confirm' && (
            <div style={{ textAlign: 'center', padding: '20px 0' }}>
              <div style={{ width: 64, height: 64, borderRadius: '50%', background: 'linear-gradient(135deg, #7c3aed, #9333ea)', display: 'flex', alignItems: 'center', justifyContent: 'center', margin: '0 auto 16px' }}>
                <Send size={28} color="#fff" />
              </div>
              <h3 style={{ fontSize: 18, fontWeight: 800, color: '#1e293b', margin: '0 0 8px' }}>¿Listo para enviar?</h3>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, margin: '20px 0', textAlign: 'left' }}>
                {[
                  { label: 'Campaña', val: title },
                  { label: 'Asunto', val: subject },
                  { label: 'Plantilla', val: selectedTemplate?.title ?? '—' },
                  { label: 'Destinatarios', val: `~${totalRecipients} seleccionado${totalRecipients !== 1 ? 's' : ''}` },
                ].map(r => (
                  <div key={r.label} style={{ background: '#f8fafc', borderRadius: 8, padding: '10px 14px' }}>
                    <div style={{ fontSize: 11, color: '#94a3b8', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{r.label}</div>
                    <div style={{ fontSize: 14, fontWeight: 600, color: '#1e293b', marginTop: 3 }}>{r.val}</div>
                  </div>
                ))}
              </div>
              <p style={{ fontSize: 13, color: '#64748b', marginTop: 8 }}>
                El email se enviará inmediatamente a través de Brevo a todos los destinatarios seleccionados.
              </p>
            </div>
          )}
        </div>
        <div className={styles.modalFooter}>
          {step === 'setup' && (
            <>
              <button className={styles.btnSecondary} onClick={onClose}>Cancelar</button>
              <button className={styles.btnPrimary} disabled={!title || !subject || !templateId} onClick={() => setStep('recipients')}>
                Siguiente
              </button>
            </>
          )}
          {step === 'recipients' && (
            <>
              <button className={styles.btnSecondary} onClick={() => setStep('setup')}>← Atrás</button>
              <button className={styles.btnSecondary} onClick={handleSaveDraft} disabled={saving}>
                Guardar borrador
              </button>
              <button className={styles.btnPrimary} disabled={totalRecipients === 0} onClick={async () => { await handleSaveDraft(); setStep('confirm') }}>
                Siguiente
              </button>
            </>
          )}
          {step === 'confirm' && (
            <>
              <button className={styles.btnSecondary} onClick={() => setStep('recipients')}>← Atrás</button>
              <button className={styles.btnPrimary} disabled={saving} onClick={handleSend}>
                {saving ? 'Enviando...' : <><Send size={14} /> Enviar ahora</>}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Main page ─────────────────────────────────────────────────────────────
type PageTab = 'campaigns' | 'templates'

export default function EmailCampaigns() {
  const [tab, setTab] = useState<PageTab>('campaigns')
  const [campaigns, setCampaigns] = useState<EmailCampaign[]>([])
  const [templates, setTemplates] = useState<EmailTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [showCampaignModal, setShowCampaignModal] = useState(false)
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [editingCampaign, setEditingCampaign] = useState<EmailCampaign | undefined>()
  const [editingTemplate, setEditingTemplate] = useState<EmailTemplate | undefined>()
  const [toast, setToast] = useState<{ msg: string; type: ToastType } | null>(null)

  const notify = (msg: string, type: ToastType = 'success') => setToast({ msg, type })

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [c, t] = await Promise.all([emailService.getCampaigns(), emailService.getTemplates()])
      setCampaigns(c)
      setTemplates(t)
    } catch { notify('Error al cargar datos', 'error') }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { load() }, [load])

  const handleSaveTemplate = async (data: Partial<EmailTemplate>) => {
    setSaving(true)
    try {
      if (editingTemplate?.id) {
        const updated = await emailService.updateTemplate(editingTemplate.id, data)
        setTemplates(ts => ts.map(t => t.id === updated.id ? updated : t))
        notify('Plantilla actualizada correctamente')
      } else {
        const created = await emailService.createTemplate(data)
        setTemplates(ts => [...ts, created])
        notify('Plantilla creada correctamente')
      }
      setShowTemplateModal(false)
      setEditingTemplate(undefined)
    } catch { notify('Error al guardar la plantilla', 'error') }
    finally { setSaving(false) }
  }

  const handleDeleteTemplate = async (id: number) => {
    if (!confirm('¿Eliminar esta plantilla?')) return
    try {
      await emailService.deleteTemplate(id)
      setTemplates(ts => ts.filter(t => t.id !== id))
      notify('Plantilla eliminada')
    } catch { notify('Error al eliminar', 'error') }
  }

  const handleSaveCampaign = async (data: Partial<EmailCampaign>): Promise<EmailCampaign> => {
    setSaving(true)
    try {
      let result: EmailCampaign
      if (editingCampaign?.id) {
        result = await emailService.updateCampaign(editingCampaign.id, data)
        setCampaigns(cs => cs.map(c => c.id === result.id ? result : c))
      } else {
        result = await emailService.createCampaign(data)
        setCampaigns(cs => [...cs, result])
      }
      return result
    } catch { notify('Error al guardar la campaña', 'error'); throw new Error('save failed') }
    finally { setSaving(false) }
  }

  const handleSendCampaign = async (id: number) => {
    setSaving(true)
    try {
      const result = await emailService.sendCampaign(id)
      await load()
      notify(`✓ Campaña enviada a ${result.sent ?? 0} destinatarios`)
    } catch { notify('Error al enviar la campaña', 'error') }
    finally { setSaving(false) }
  }

  const handleDeleteCampaign = async (id: number) => {
    if (!confirm('¿Eliminar esta campaña?')) return
    try {
      await emailService.deleteCampaign(id)
      setCampaigns(cs => cs.filter(c => c.id !== id))
      notify('Campaña eliminada')
    } catch { notify('Error al eliminar', 'error') }
  }

  const openNewCampaign = () => { setEditingCampaign(undefined); setShowCampaignModal(true) }
  const openNewTemplate = () => { setEditingTemplate(undefined); setShowTemplateModal(true) }

  return (
    <div className={styles.page}>
      {/* Header */}
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>
            <Mail size={22} style={{ verticalAlign: 'middle', marginRight: 8, color: '#7c3aed' }} />
            Email Marketing
          </h1>
          <p className={styles.pageSubtitle}>Diseña, gestiona y envía campañas de email via Brevo</p>
        </div>
        <button className={styles.btnPrimary} onClick={tab === 'campaigns' ? openNewCampaign : openNewTemplate}>
          <Plus size={16} />
          {tab === 'campaigns' ? 'Nueva campaña' : 'Nueva plantilla'}
        </button>
      </div>

      {/* Tabs */}
      <div className={styles.tabs}>
        <button className={`${styles.tab} ${tab === 'campaigns' ? styles.active : ''}`} onClick={() => setTab('campaigns')}>
          <Send size={14} style={{ verticalAlign: 'middle', marginRight: 5 }} />
          Campañas ({campaigns.length})
        </button>
        <button className={`${styles.tab} ${tab === 'templates' ? styles.active : ''}`} onClick={() => setTab('templates')}>
          <LayoutTemplate size={14} style={{ verticalAlign: 'middle', marginRight: 5 }} />
          Plantillas ({templates.length})
        </button>
      </div>

      {/* Content */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 48, color: '#94a3b8' }}>Cargando...</div>
      ) : tab === 'campaigns' ? (
        campaigns.length === 0
          ? (
            <div className={styles.empty}>
              <Send size={48} />
              <h3>No hay campañas todavía</h3>
              <p>Crea tu primera campaña para empezar a comunicarte con tus usuarios.</p>
              <button className={styles.btnPrimary} onClick={openNewCampaign}><Plus size={14} /> Nueva campaña</button>
            </div>
          )
          : (
            <div className={styles.campaignGrid}>
              {campaigns.map(c => (
                <div key={c.id} className={styles.campaignCard}>
                  <div className={styles.cardHeader}>
                    <div>
                      <p className={styles.cardTitle}>{c.title}</p>
                      <p className={styles.cardSubtitle}>{c.subject}</p>
                    </div>
                    <StatusBadge status={c.status} />
                  </div>
                  <div className={styles.cardStats}>
                    <div className={styles.stat}>
                      <span className={styles.statVal}>{c.recipients ?? 0}</span>
                      <span className={styles.statLbl}>Enviados</span>
                    </div>
                    <div className={styles.stat}>
                      <span className={styles.statVal}>{c.open_rate ? `${c.open_rate.toFixed(1)}%` : '—'}</span>
                      <span className={styles.statLbl}>Abiertos</span>
                    </div>
                    <div className={styles.stat}>
                      <span className={styles.statVal}>{c.click_rate ? `${c.click_rate.toFixed(1)}%` : '—'}</span>
                      <span className={styles.statLbl}>Clicks</span>
                    </div>
                  </div>
                  {c.sent_at && (
                    <p style={{ fontSize: 11, color: '#94a3b8', margin: '8px 0 0' }}>
                      Enviada: {new Date(c.sent_at).toLocaleDateString('es-AR', { day: '2-digit', month: 'short', year: 'numeric' })}
                    </p>
                  )}
                  <div className={styles.cardActions}>
                    {c.status !== 'sent' && (
                      <button className={styles.btnPrimary} style={{ fontSize: 12, padding: '7px 14px' }} onClick={() => handleSendCampaign(c.id!)} disabled={saving}>
                        <Send size={13} /> Enviar
                      </button>
                    )}
                    {c.status !== 'sent' && (
                      <button className={styles.btnSecondary} onClick={() => { setEditingCampaign(c); setShowCampaignModal(true) }}>
                        <Edit3 size={13} /> Editar
                      </button>
                    )}
                    <button className={styles.btnDanger} onClick={() => handleDeleteCampaign(c.id!)}>
                      <Trash2 size={13} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )
      ) : (
        templates.length === 0
          ? (
            <div className={styles.empty}>
              <LayoutTemplate size={48} />
              <h3>No hay plantillas</h3>
              <p>Las plantillas definen el diseño visual de tus emails. Crea la primera.</p>
              <button className={styles.btnPrimary} onClick={openNewTemplate}><Plus size={14} /> Nueva plantilla</button>
            </div>
          )
          : (
            <div className={styles.templateGrid}>
              {templates.map(t => (
                <div key={t.id} className={styles.templateCard}>
                  <div className={styles.templatePreview}>
                    <Mail size={36} color="#a78bfa" />
                    <div style={{ position: 'absolute', top: 8, right: 8, fontSize: 10, background: '#7c3aed', color: '#fff', padding: '2px 8px', borderRadius: 20, fontWeight: 700, textTransform: 'uppercase' }}>
                      {t.type}
                    </div>
                  </div>
                  <div className={styles.templateBody}>
                    <p className={styles.templateName}>{t.title}</p>
                    <p className={styles.templateMeta}>Asunto: {t.subject}</p>
                    <div className={styles.templateActions}>
                      <button className={styles.btnSecondary} style={{ fontSize: 12, padding: '6px 12px' }} onClick={() => { setEditingTemplate(t); setShowTemplateModal(true) }}>
                        <Edit3 size={12} /> Editar
                      </button>
                      <button className={styles.btnDanger} style={{ fontSize: 12, padding: '6px 12px' }} onClick={() => handleDeleteTemplate(t.id!)}>
                        <Trash2 size={12} />
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )
      )}

      {/* Modals */}
      {showTemplateModal && (
        <TemplateModal
          initial={editingTemplate}
          onSave={handleSaveTemplate}
          onClose={() => { setShowTemplateModal(false); setEditingTemplate(undefined) }}
          saving={saving}
        />
      )}
      {showCampaignModal && (
        <CampaignModal
          initial={editingCampaign}
          templates={templates}
          onSave={handleSaveCampaign}
          onSend={handleSendCampaign}
          onClose={() => { setShowCampaignModal(false); setEditingCampaign(undefined) }}
          saving={saving}
        />
      )}

      {/* Toast */}
      {toast && <Toast msg={toast.msg} type={toast.type} onClose={() => setToast(null)} />}
    </div>
  )
}
