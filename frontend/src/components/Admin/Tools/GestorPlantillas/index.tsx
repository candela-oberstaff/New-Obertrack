import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Plus, Edit3, Trash2, LayoutTemplate, Mail, Search, Send } from 'lucide-react';
import { emailService, EmailTemplate } from '../../../../services/emailService';
import TemplateEditor from './components/TemplateEditor';
import Pagination from '../Common/Pagination';
import RecipientSelector from '../../../../pages/Email/RecipientSelector';
import styles from './GestorPlantillas.module.css';

type ToastType = 'success' | 'error';

function Toast({ msg, type, onClose }: { msg: string; type: ToastType; onClose: () => void }) {
  useEffect(() => {
    const t = setTimeout(onClose, 3000);
    return () => clearTimeout(t);
  }, [onClose]);

  return (
    <div className={`${styles.toast} ${type === 'success' ? styles.toastSuccess : styles.toastError}`}>
      {msg}
    </div>
  );
}

const GestorPlantillas: React.FC = () => {
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [showEditor, setShowEditor] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<EmailTemplate | undefined>();
  const [toast, setToast] = useState<{ msg: string; type: ToastType } | null>(null);
  const [search, setSearch] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [page, setPage] = useState(1);
  const [showSendModal, setShowSendModal] = useState(false);
  const [sending, setSending] = useState(false);
  const [sendTemplateId, setSendTemplateId] = useState<number | ''>('');
  const [sendRecipients, setSendRecipients] = useState<{ userIds: number[]; groupIds: number[]; expressContacts: Array<{ name: string; email: string }> }>({ userIds: [], groupIds: [], expressContacts: [] });

  const ITEMS_PER_PAGE = 12;

  const notify = (msg: string, type: ToastType = 'success') => setToast({ msg, type });

  const loadTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const data = await emailService.getTemplates();
      setTemplates(data);
    } catch {
      notify('Error al cargar plantillas', 'error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadTemplates();
  }, [loadTemplates]);

  useEffect(() => { setPage(1); }, [search, dateFrom, dateTo]);

  const gestorTemplates = useMemo(() => {
    return templates.filter(t => t.type === 'gestor');
  }, [templates]);

  const filtered = useMemo(() => {
    return gestorTemplates.filter(t => {
      const q = search.toLowerCase();
      if (q && !t.title.toLowerCase().includes(q) && !t.subject.toLowerCase().includes(q)) return false;
      if (t.created_at) {
        const d = new Date(t.created_at);
        if (dateFrom && d < new Date(dateFrom)) return false;
        if (dateTo) {
          const end = new Date(dateTo);
          end.setHours(23, 59, 59, 999);
          if (d > end) return false;
        }
      }
      return true;
    });
  }, [gestorTemplates, search, dateFrom, dateTo]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE));
  const safePage = Math.min(page, totalPages);
  const paginated = filtered.slice((safePage - 1) * ITEMS_PER_PAGE, safePage * ITEMS_PER_PAGE);

  const handleCreate = () => {
    setEditingTemplate(undefined);
    setShowEditor(true);
  };

  const handleEdit = (template: EmailTemplate) => {
    setEditingTemplate(template);
    setShowEditor(true);
  };

  const handleDelete = async (id: number) => {
    if (!confirm('¿Eliminar esta plantilla?')) return;
    try {
      await emailService.deleteTemplate(id);
      setTemplates(ts => ts.filter(t => t.id !== id));
      notify('Plantilla eliminada correctamente');
    } catch {
      notify('Error al eliminar la plantilla', 'error');
    }
  };

  const handleSave = async (data: Partial<EmailTemplate>) => {
    try {
      if (editingTemplate?.id) {
        const updated = await emailService.updateTemplate(editingTemplate.id, data);
        setTemplates(ts => ts.map(t => t.id === updated.id ? updated : t));
        notify('Plantilla actualizada correctamente');
      } else {
        const created = await emailService.createTemplate(data);
        setTemplates(ts => [...ts, created]);
        notify('Plantilla creada correctamente');
      }
      setShowEditor(false);
      setEditingTemplate(undefined);
    } catch {
      notify('Error al guardar la plantilla', 'error');
    }
  };

  const handleCloseEditor = () => {
    setShowEditor(false);
    setEditingTemplate(undefined);
  };

  const handleSendTemplate = async () => {
    if (!sendTemplateId) return;
    setSending(true);
    try {
      const recipientList = JSON.stringify({
        userIds: sendRecipients.userIds,
        groupIds: sendRecipients.groupIds,
        expressContacts: sendRecipients.expressContacts,
      });
      const result = await emailService.sendTemplate(sendTemplateId, recipientList);
      notify(`Plantilla enviada a ${result.sent ?? 0} destinatario${result.sent !== 1 ? 's' : ''}`);
      setShowSendModal(false);
      setSendTemplateId('');
      setSendRecipients({ userIds: [], groupIds: [], expressContacts: [] });
    } catch {
      notify('Error al enviar la plantilla', 'error');
    } finally {
      setSending(false);
    }
  };

  if (showEditor) {
    return (
      <TemplateEditor
        initial={editingTemplate}
        onSave={handleSave}
        onClose={handleCloseEditor}
      />
    );
  }

  return (
    <div className={styles.page}>
      {/* Header */}
      <div className={styles.header}>
        <div className={styles.headerInfo}>
          <div className={styles.headerIcon}>
            <LayoutTemplate size={20} />
          </div>
          <div>
            <h1 className={styles.title}>Gestor de Plantillas</h1>
            <p className={styles.subtitle}>
              Crea y administra plantillas de email para tus campañas
            </p>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className={styles.btnPrimary} style={{ background: '#0ea5e9' }} onClick={() => { setSendTemplateId(''); setSendRecipients({ userIds: [], groupIds: [], expressContacts: [] }); setShowSendModal(true); }}>
            <Send size={16} />
            Enviar plantilla
          </button>
          <button className={styles.btnPrimary} onClick={handleCreate}>
            <Plus size={16} />
            Nueva plantilla
          </button>
        </div>
      </div>

      {/* Search & Filter Bar */}
      {!loading && gestorTemplates.length > 0 && (
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: 16, flexWrap: 'wrap' }}>
          <div style={{ position: 'relative', flex: 1, minWidth: 200 }}>
            <Search size={15} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8', pointerEvents: 'none' }} />
            <input
              type="text"
              placeholder="Buscar por nombre o asunto..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              style={{ width: '100%', border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px 8px 34px', fontSize: 13, outline: 'none', color: '#1e293b', background: '#fff', boxSizing: 'border-box' }}
            />
          </div>
          <input type="date" value={dateFrom} onChange={e => setDateFrom(e.target.value)}
            style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px', fontSize: 13, outline: 'none', color: '#1e293b', background: '#fff' }}
            title="Desde" />
          <input type="date" value={dateTo} onChange={e => setDateTo(e.target.value)}
            style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px', fontSize: 13, outline: 'none', color: '#1e293b', background: '#fff' }}
            title="Hasta" />
          {(dateFrom || dateTo) && (
            <button onClick={() => { setDateFrom(''); setDateTo(''); }}
              style={{ padding: '8px 12px', border: '1px solid #e2e8f0', borderRadius: 8, background: '#fff', color: '#ef4444', fontSize: 12, fontWeight: 600, cursor: 'pointer', whiteSpace: 'nowrap' }}
              title="Limpiar filtros de fecha">✕ Limpiar</button>
          )}
          <span style={{ fontSize: 12, color: '#94a3b8', whiteSpace: 'nowrap' }}>{filtered.length} de {gestorTemplates.length}</span>
        </div>
      )}

      {/* Content */}
      {loading ? (
        <div className={styles.loading}>Cargando plantillas...</div>
      ) : filtered.length === 0 ? (
        <div className={styles.empty}>
          <LayoutTemplate size={48} />
          <h3>{gestorTemplates.length === 0 ? 'No hay plantillas' : 'Sin resultados'}</h3>
          <p>{gestorTemplates.length === 0 ? 'Crea tu primera plantilla para empezar a diseñar emails profesionales.' : 'No se encontraron plantillas con los filtros aplicados.'}</p>
          {gestorTemplates.length === 0 && (
            <button className={styles.btnPrimary} onClick={handleCreate}>
              <Plus size={14} />
              Nueva plantilla
            </button>
          )}
        </div>
      ) : (
        <>
          <div className={styles.grid}>
            {paginated.map(template => (
              <div key={template.id} className={styles.card}>
                <div className={styles.cardPreview}>
                  <Mail size={32} color="#a78bfa" />
                  <div className={styles.cardTypeBadge}>{template.type}</div>
                </div>
                <div className={styles.cardBody}>
                  <h3 className={styles.cardTitle}>{template.title}</h3>
                  <p className={styles.cardSubject}>Asunto: {template.subject}</p>
                  <div className={styles.cardMeta}>
                    {template.created_at && (
                      <span>Creada: {new Date(template.created_at).toLocaleDateString('es-AR')}</span>
                    )}
                  </div>
                </div>
                <div className={styles.cardActions}>
                  <button
                    className={styles.btnSecondary}
                    onClick={() => handleEdit(template)}
                  >
                    <Edit3 size={13} />
                    Editar
                  </button>
                  <button
                    className={styles.btnDanger}
                    onClick={() => template.id && handleDelete(template.id)}
                  >
                    <Trash2 size={13} />
                  </button>
                </div>
              </div>
            ))}
          </div>
          <Pagination currentPage={safePage} totalPages={totalPages} onPageChange={setPage} />
        </>
      )}

      {/* Send Modal */}
      {showSendModal && (
        <div className={styles.modalBackdrop} onClick={() => !sending && setShowSendModal(false)}>
          <div className={styles.modal} onClick={e => e.stopPropagation()}>
            <h2>Enviar plantilla</h2>

            <label className={styles.modalLabel}>Plantilla</label>
            <select className={styles.modalSelect} value={sendTemplateId} onChange={e => setSendTemplateId(e.target.value ? Number(e.target.value) : '')}>
              <option value="">Seleccionar plantilla...</option>
              {gestorTemplates.map(t => (
                <option key={t.id} value={t.id}>{t.title}</option>
              ))}
            </select>
            {!sendTemplateId && <p style={{ margin: '-16px 0 16px', fontSize: 12, color: '#ef4444' }}>Seleccioná una plantilla para poder enviar</p>}

            <label className={styles.modalLabel}>Destinatarios</label>
            <RecipientSelector
              value={sendRecipients}
              onChange={setSendRecipients}
            />

            <div className={styles.modalActions}>
              <button className={styles.btnCancel} onClick={() => setShowSendModal(false)} disabled={sending}>Cancelar</button>
              <button className={styles.btnPrimary} onClick={handleSendTemplate} disabled={!sendTemplateId || sending}>
                {sending ? 'Enviando...' : 'Enviar'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Toast */}
      {toast && <Toast msg={toast.msg} type={toast.type} onClose={() => setToast(null)} />}
    </div>
  );
};

export default GestorPlantillas;
