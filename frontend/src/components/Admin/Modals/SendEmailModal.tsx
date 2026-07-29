import React, { useEffect, useMemo, useState } from 'react';
import { X, Mail, Send, LayoutTemplate, PenLine, Users, CheckCircle2, AlertTriangle, Loader2, ClipboardList, Megaphone, KeyRound, Link2 } from 'lucide-react';
import { emailService, EmailTemplate } from '../../../services/emailService';
import { surveyService, Survey } from '../../../services/surveyService';
import { adminService } from '../../../services/admin.service';
import { blocksFromJSON, templateToHTML } from '../Tools/Common/emailTypes';
import { renderWithExamples } from '../Tools/Common/emailVariables';
import { VariableMenu, useEmailVariables } from '../Tools/Common/VariableMenu';
import { Select } from '../../ui/Select';
import { useCloseGuard } from '../../ui/useCloseGuard';
import { TestSendButton } from '../Tools/Common/TestSendButton';

/**
 * Envío masivo desde el panel de usuarios, apoyado en Tools.
 *
 * Los destinatarios viajan como IDs de usuario, no como correos sueltos: así el
 * backend los resuelve contra la base de datos y puede personalizar cada correo
 * con todas las variables ({{primer_nombre}}, {{empresa}}, {{cargo}}, ...).
 */

export interface EmailRecipientUser {
  id: number;
  name: string;
  email: string;
}

interface SendEmailModalProps {
  /** Usuarios marcados con la casilla. */
  selected: EmailRecipientUser[];
  /** Todos los que coinciden con los filtros activos de la tabla. */
  filtered: EmailRecipientUser[];
  onClose: () => void;
}

type Scope = 'selected' | 'filtered';
type Mode = 'content' | 'quick' | 'access';

/**
 * Forma de entregar el acceso. La contraseña temporal que genera la importación
 * masiva NO se puede reenviar (se guarda hasheada), así que ambos modos emiten
 * un acceso nuevo en vez de repetir el anterior.
 */
type AccessMode = 'invite' | 'password';

const ACCESS_MODES: { value: AccessMode; label: string; icon: React.ReactNode; detail: string; warn?: string }[] = [
  {
    value: 'invite',
    label: 'Enlace para crear su contraseña',
    icon: <Link2 size={14} />,
    detail: 'Recibe un enlace personal, válido 24 horas, para elegir su propia contraseña. No viaja ninguna clave por correo.',
  },
  {
    value: 'password',
    label: 'Contraseña temporal nueva',
    icon: <KeyRound size={14} />,
    detail: 'Se genera una clave nueva y se envía en el correo.',
    warn: 'La contraseña queda guardada para siempre en su bandeja, y las del CSV que descargaste al importar dejarán de funcionar.',
  },
];

/**
 * Tools guarda cosas de naturaleza distinta y todas son enviables. Hay que
 * distinguirlas en el selector: una plantilla y una campaña se envían como
 * correo diseñado, mientras que una encuesta manda un enlace para responder.
 */
type ContentKind = 'template' | 'campaign' | 'survey';

interface ContentItem {
  /** Clave única del selector: el id colisiona entre plantillas y encuestas. */
  key: string;
  id: number;
  kind: ContentKind;
  title: string;
  subject: string;
  /** Bloques de la plantilla; vacío en las encuestas. */
  content: string;
}

const KIND_META: Record<ContentKind, { label: string; color: string; icon: React.ReactNode; hint: string }> = {
  template: {
    label: 'Plantilla',
    color: '#7c3aed',
    icon: <LayoutTemplate size={13} />,
    hint: 'Se envía el correo diseñado, con las variables resueltas para cada persona.',
  },
  campaign: {
    label: 'Campaña',
    color: '#2563eb',
    icon: <Megaphone size={13} />,
    hint: 'Diseño guardado desde el editor de campañas. Se envía como correo, sin registrarlo como campaña ni afectar sus métricas.',
  },
  survey: {
    label: 'Encuesta',
    color: '#16a34a',
    icon: <ClipboardList size={13} />,
    hint: 'Se envía el enlace para responder, por correo y como notificación en la app. No usa variables, y si la encuesta estaba en borrador pasa a activa.',
  },
};

interface SendResult {
  sent: number;
  total: number;
  errors?: string[];
}

/** Texto plano a HTML, respetando los saltos de línea que escribió el usuario. */
function plainTextToHTML(text: string): string {
  return text
    .split(/\n{2,}/)
    .map(p => `<p style="margin:0 0 16px 0;">${p.replace(/\n/g, '<br />')}</p>`)
    .join('');
}

const SendEmailModal: React.FC<SendEmailModalProps> = ({ selected, filtered, onClose }) => {
  const [scope, setScope] = useState<Scope>(selected.length > 0 ? 'selected' : 'filtered');
  const [mode, setMode] = useState<Mode>('content');
  const [items, setItems] = useState<ContentItem[]>([]);
  const [itemKey, setItemKey] = useState<string>('');
  const [accessMode, setAccessMode] = useState<AccessMode>('invite');
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<SendResult | null>(null);
  const [error, setError] = useState('');

  const { variables, bindField, insertVariable, activeFieldLabel, dragSourceProps, findUnknown } = useEmailVariables();

  // Solo hay algo que perder si se redactó un mensaje o se eligió qué enviar.
  // Con el modal recién abierto cierra sin preguntar.
  const requestClose = useCloseGuard(
    () => !result && (subject.trim() !== '' || body.trim() !== '' || itemKey !== ''),
    onClose,
    {
      title: '¿Descartar este correo?',
      message: 'Todavía no se ha enviado. Si cierras ahora se perderá lo que preparaste.',
    }
  );

  useEffect(() => {
    let cancelled = false;

    // Plantillas y encuestas viven en endpoints distintos, pero para quien
    // envía son la misma lista de "contenido de Tools".
    Promise.all([
      emailService.getTemplates().catch(() => [] as EmailTemplate[]),
      surveyService.getSurveys().catch(() => [] as Survey[]),
    ]).then(([templates, surveys]) => {
      if (cancelled) return;

      const fromTemplates: ContentItem[] = (templates || [])
        .filter(t => t.is_active !== false && t.id)
        .map(t => ({
          key: `template-${t.id}`,
          id: t.id as number,
          // 'campaign' son los diseños guardados desde el editor de campañas.
          kind: t.type === 'campaign' ? 'campaign' : 'template',
          title: t.title,
          subject: t.subject || t.title,
          content: t.content,
        }));

      // Una encuesta cerrada ya no admite respuestas: no tiene sentido enviarla.
      const fromSurveys: ContentItem[] = (Array.isArray(surveys) ? surveys : [])
        .filter((s: Survey) => s.id && s.status !== 'closed')
        .map((s: Survey) => ({
          key: `survey-${s.id}`,
          id: s.id as number,
          kind: 'survey' as ContentKind,
          title: s.title,
          subject: s.title,
          content: '',
        }));

      const order: Record<ContentKind, number> = { template: 0, campaign: 1, survey: 2 };
      setItems([...fromTemplates, ...fromSurveys].sort(
        (a, b) => order[a.kind] - order[b.kind] || a.title.localeCompare(b.title)
      ));
    });

    return () => { cancelled = true; };
  }, []);

  // Sin correo no hay a quién enviar: se descartan antes de contar, para que el
  // número del botón sea el real.
  const recipients = useMemo(
    () => (scope === 'selected' ? selected : filtered).filter(u => !!u.email),
    [scope, selected, filtered]
  );

  const item = items.find(i => i.key === itemKey);

  const options = useMemo(
    () => items.map(i => ({
      value: i.key,
      // El tipo va en la etiqueta, no solo en el color: se lee sin depender
      // de distinguir tonos.
      label: `${KIND_META[i.kind].label} · ${i.title}`,
      color: KIND_META[i.kind].color,
    })),
    [items]
  );

  // Una encuesta no se previsualiza aquí: su correo lo arma el backend con el
  // enlace para responder.
  const previewHTML = useMemo(() => {
    if (mode === 'access') return '';
    if (mode === 'quick') return renderWithExamples(plainTextToHTML(body), variables);
    if (!item || item.kind === 'survey') return '';
    return renderWithExamples(templateToHTML(blocksFromJSON(item.content)), variables);
  }, [mode, item, body, variables]);

  const unknownVars = useMemo(
    () => findUnknown(mode === 'quick' ? `${subject}\n${body}` : (item?.subject ?? '')),
    [mode, item, subject, body, findUnknown]
  );

  const canSend = recipients.length > 0 && !sending && (
    mode === 'access' ? true
      : mode === 'quick' ? subject.trim() !== '' && body.trim() !== ''
      : !!item
  );

  // La prueba no aplica al modo Acceso ni a las encuestas: ese correo lo
  // compone el backend con un enlace personal que no tiene sentido simular.
  const testPayload = () => {
    if (mode === 'quick') {
      if (!subject.trim() || !body.trim()) return null;
      return { subject, html_content: plainTextToHTML(body) };
    }
    if (mode === 'content' && item && item.kind !== 'survey') {
      return { subject: item.subject, template_id: item.id };
    }
    return null;
  };

  const handleSend = async () => {
    if (!canSend) return;
    setSending(true);
    setError('');
    try {
      const userIds = recipients.map(u => u.id);
      let res: any;

      if (mode === 'access') {
        const r = await adminService.sendAccessEmails({ user_ids: userIds, mode: accessMode });
        res = {
          sent: r.sent,
          total: r.total,
          errors: (r.failed || []).map(f => `${f.email || `#${f.id}`}: ${f.error}`),
        };
      } else if (mode === 'quick') {
        res = await emailService.sendQuickEmailBulk({
          recipient_list: JSON.stringify({ userIds }),
          subject,
          html_content: plainTextToHTML(body),
        });
      } else if (item!.kind === 'survey') {
        // Las encuestas usan una lista plana de IDs, no el formato híbrido.
        res = await surveyService.sendSurveyToRecipients(item!.id, userIds);
      } else {
        res = await emailService.sendTemplate(item!.id, JSON.stringify({ userIds }));
      }

      setResult({
        sent: res.sent ?? 0,
        total: res.total ?? recipients.length,
        errors: res.errors,
      });
    } catch (e: any) {
      // Un mensaje genérico obliga a abrir la consola para saber qué pasó. Si
      // el backend no explicó el fallo, al menos se dice qué respondió.
      const status = e?.response?.status;
      setError(
        e?.response?.data?.error
        ?? (status === 404
          ? 'El servidor no reconoce esta acción (404). Puede que el backend no esté actualizado.'
          : status === 403
            ? 'No tienes permisos para esta acción.'
            : status
              ? `El servidor respondió con un error ${status}.`
              : 'No hubo respuesta del servidor. Revisa que el backend esté disponible.')
      );
    } finally {
      setSending(false);
    }
  };

  // ─── Estilos ──────────────────────────────────────────────────────────────
  const overlay: React.CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(15,23,42,0.55)', zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 };
  const modal: React.CSSProperties = { background: '#fff', borderRadius: 16, width: '100%', maxWidth: 780, maxHeight: '92vh', display: 'flex', flexDirection: 'column', boxShadow: '0 25px 50px rgba(0,0,0,0.25)', overflow: 'hidden' };
  const label: React.CSSProperties = { fontSize: 10, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.08em' };
  const input: React.CSSProperties = { width: '100%', border: '1px solid #e2e8f0', borderRadius: 8, padding: '9px 12px', fontSize: 13, outline: 'none', color: '#1e293b', fontFamily: 'inherit' };
  const tab = (active: boolean): React.CSSProperties => ({ display: 'flex', alignItems: 'center', gap: 6, padding: '8px 14px', borderRadius: 8, border: 'none', cursor: 'pointer', fontSize: 12, fontWeight: 700, fontFamily: 'inherit', background: active ? '#fff' : 'transparent', color: active ? '#1e293b' : '#64748b', boxShadow: active ? '0 1px 3px rgba(0,0,0,0.08)' : 'none' });
  const scopeCard = (active: boolean): React.CSSProperties => ({ display: 'flex', alignItems: 'center', gap: 8, flex: 1, padding: '10px 12px', borderRadius: 10, cursor: 'pointer', fontSize: 12, border: `1px solid ${active ? '#7c3aed' : '#e2e8f0'}`, background: active ? '#f5f3ff' : '#fff', color: active ? '#6d28d9' : '#475569', fontWeight: 600 });

  // ─── Resumen del envío ────────────────────────────────────────────────────
  if (result) {
    const failed = result.total - result.sent;
    return (
      <div style={overlay} onClick={onClose}>
        <div style={{ ...modal, maxWidth: 460 }} onClick={e => e.stopPropagation()}>
          <div style={{ padding: 32, textAlign: 'center' }}>
            {/* display:block + margen automático: los svg son bloque en esta app,
                así que text-align no basta para centrarlos. */}
            {failed === 0
              ? <CheckCircle2 size={44} style={{ color: '#16a34a', display: 'block', margin: '0 auto 12px' }} />
              : <AlertTriangle size={44} style={{ color: '#f59e0b', display: 'block', margin: '0 auto 12px' }} />}
            <h3 style={{ margin: '0 0 6px', fontSize: 17, fontWeight: 800, color: '#1e293b' }}>
              Enviado a {result.sent} de {result.total}
            </h3>
            <p style={{ margin: 0, fontSize: 13, color: '#64748b' }}>
              {failed > 0
                ? `${failed} envío(s) fallaron.`
                : mode === 'access'
                  ? (accessMode === 'invite'
                      ? 'Cada persona recibió su enlace para crear la contraseña. Caduca en 24 horas.'
                      : 'Cada persona recibió su contraseña nueva. La anterior ya no funciona.')
                  : 'Cada persona recibió el correo con sus propios datos.'}
            </p>
            {result.errors && result.errors.length > 0 && (
              <ul style={{ textAlign: 'left', margin: '14px 0 0', padding: '10px 12px 10px 28px', background: '#fffbeb', border: '1px solid #fde68a', borderRadius: 8, fontSize: 11, color: '#b45309', maxHeight: 140, overflowY: 'auto' }}>
                {result.errors.map((err, i) => <li key={i}>{err}</li>)}
              </ul>
            )}
            <button onClick={onClose} style={{ marginTop: 20, padding: '10px 24px', borderRadius: 10, border: 'none', background: '#7c3aed', color: '#fff', fontSize: 13, fontWeight: 700, cursor: 'pointer', fontFamily: 'inherit' }}>
              Cerrar
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={overlay} onClick={requestClose}>
      <div style={modal} onClick={e => e.stopPropagation()}>
        {/* Cabecera */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '18px 24px', borderBottom: '1px solid #f1f5f9' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ width: 34, height: 34, borderRadius: 9, background: '#f5f3ff', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#7c3aed' }}>
              <Mail size={17} />
            </span>
            <div>
              <h2 style={{ margin: 0, fontSize: 16, fontWeight: 800, color: '#1e293b' }}>Enviar correo</h2>
              <p style={{ margin: 0, fontSize: 11, color: '#94a3b8' }}>Usa las plantillas y variables de Tools</p>
            </div>
          </div>
          <button onClick={requestClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8', padding: 4 }}>
            <X size={19} />
          </button>
        </div>

        <div style={{ padding: 24, overflowY: 'auto', flex: 1, display: 'flex', flexDirection: 'column', gap: 18 }}>
          {/* Destinatarios */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <span style={label}>Destinatarios</span>
            <div style={{ display: 'flex', gap: 10 }}>
              <label style={scopeCard(scope === 'selected')}>
                <input type="radio" checked={scope === 'selected'} onChange={() => setScope('selected')} disabled={selected.length === 0} style={{ cursor: 'pointer' }} />
                <Users size={14} /> Seleccionados ({selected.filter(u => u.email).length})
              </label>
              <label style={scopeCard(scope === 'filtered')}>
                <input type="radio" checked={scope === 'filtered'} onChange={() => setScope('filtered')} style={{ cursor: 'pointer' }} />
                <Users size={14} /> Todos los del filtro ({filtered.filter(u => u.email).length})
              </label>
            </div>
          </div>

          {/* Modo */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
            <div style={{ display: 'flex', gap: 4, background: '#f1f5f9', padding: 4, borderRadius: 10 }}>
              <button type="button" style={tab(mode === 'content')} onClick={() => setMode('content')}>
                <LayoutTemplate size={13} /> Contenido de Tools
              </button>
              <button type="button" style={tab(mode === 'quick')} onClick={() => setMode('quick')}>
                <PenLine size={13} /> Mensaje rápido
              </button>
              <button type="button" style={tab(mode === 'access')} onClick={() => setMode('access')}>
                <KeyRound size={13} /> Acceso
              </button>
            </div>
            {mode === 'quick' && (
              <VariableMenu
                size="sm"
                variables={variables}
                onInsert={insertVariable}
                activeFieldLabel={activeFieldLabel}
                dragSourceProps={dragSourceProps}
                unknown={unknownVars}
              />
            )}
          </div>

          {mode === 'access' ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <label style={label}>Cómo entregar el acceso</label>
              {ACCESS_MODES.map(m => {
                const active = accessMode === m.value;
                return (
                  <label
                    key={m.value}
                    style={{
                      display: 'flex', alignItems: 'flex-start', gap: 10, padding: '11px 13px', borderRadius: 10,
                      cursor: 'pointer', border: `1px solid ${active ? '#7c3aed' : '#e2e8f0'}`,
                      background: active ? '#f5f3ff' : '#fff',
                    }}
                  >
                    <input type="radio" checked={active} onChange={() => setAccessMode(m.value)} style={{ marginTop: 3, cursor: 'pointer' }} />
                    <span style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, fontWeight: 700, color: active ? '#6d28d9' : '#334155' }}>
                        {m.icon} {m.label}
                      </span>
                      <span style={{ fontSize: 11, color: '#64748b', lineHeight: 1.5 }}>{m.detail}</span>
                      {m.warn && active && (
                        <span style={{
                          display: 'flex', alignItems: 'flex-start', gap: 5, marginTop: 3, fontSize: 10.5, color: '#b45309',
                          background: '#fffbeb', border: '1px solid #fde68a', borderRadius: 6, padding: '5px 7px', lineHeight: 1.5,
                        }}>
                          <AlertTriangle size={12} style={{ flexShrink: 0, marginTop: 1 }} /> {m.warn}
                        </span>
                      )}
                    </span>
                  </label>
                );
              })}
              <p style={{ margin: 0, fontSize: 11, color: '#94a3b8', lineHeight: 1.5 }}>
                La contraseña generada al importar no se puede reenviar: se guarda cifrada y solo se mostró una vez.
                Ambas opciones emiten un acceso nuevo.
              </p>
            </div>
          ) : mode === 'content' ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <label style={label}>Qué enviar</label>
              <Select
                fullWidth
                searchable
                options={options}
                value={itemKey}
                onChange={v => setItemKey(String(v))}
                placeholder="— Elige plantilla, campaña o encuesta —"
                ariaLabel="Contenido a enviar"
              />
              {item && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, padding: '10px 12px', borderRadius: 10, background: '#f8fafc', border: '1px solid #eef2f7' }}>
                  <span style={{
                    display: 'inline-flex', alignItems: 'center', gap: 5, alignSelf: 'flex-start',
                    fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em',
                    color: KIND_META[item.kind].color, background: '#fff',
                    border: `1px solid ${KIND_META[item.kind].color}33`, borderRadius: 20, padding: '3px 9px',
                  }}>
                    {KIND_META[item.kind].icon} {KIND_META[item.kind].label}
                  </span>
                  <p style={{ margin: 0, fontSize: 11, color: '#64748b', lineHeight: 1.5 }}>
                    {KIND_META[item.kind].hint}
                  </p>
                  {item.kind !== 'survey' && (
                    <p style={{ margin: 0, fontSize: 11, color: '#64748b' }}>
                      Asunto: <strong style={{ color: '#334155' }}>{item.subject}</strong>
                    </p>
                  )}
                </div>
              )}
              {items.length === 0 && !error && (
                <p style={{ margin: 0, fontSize: 11, color: '#94a3b8' }}>
                  Todavía no hay nada que enviar. Crea plantillas o encuestas en Tools.
                </p>
              )}
            </div>
          ) : (
            <>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <label style={label}>Asunto</label>
                <input
                  style={input}
                  value={subject}
                  onChange={e => setSubject(e.target.value)}
                  placeholder="Ej: Novedades para {{primer_nombre}}"
                  {...bindField(setSubject, 'Asunto del correo')}
                />
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <label style={label}>Mensaje</label>
                <textarea
                  style={{ ...input, minHeight: 150, resize: 'vertical', lineHeight: 1.6 }}
                  value={body}
                  onChange={e => setBody(e.target.value)}
                  placeholder={'Hola {{primer_nombre}},\n\nEscribe aquí tu mensaje...'}
                  {...bindField(setBody, 'Mensaje', { isDefault: true })}
                />
              </div>
            </>
          )}

          {/* Vista previa */}
          {previewHTML && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={label}>Vista previa</span>
                <span style={{ fontSize: 10, fontWeight: 700, color: '#7c3aed', background: '#f5f3ff', padding: '2px 8px', borderRadius: 20, textTransform: 'uppercase', letterSpacing: '0.05em' }}
                  title="Cada persona recibirá el correo con sus propios datos.">
                  variables de ejemplo
                </span>
              </div>
              <iframe
                title="Vista previa del correo"
                sandbox=""
                srcDoc={`<!DOCTYPE html><html><head><meta charset="utf-8"><style>body{margin:0;padding:12px;background:#f1f5f9;font-family:sans-serif;}img{max-width:100%}</style></head><body>${previewHTML}</body></html>`}
                style={{ width: '100%', height: 220, border: '1px solid #e2e8f0', borderRadius: 10, background: '#f1f5f9' }}
              />
            </div>
          )}

          {error && (
            <p style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: '#b91c1c', background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 8, padding: '8px 10px' }}>
              <AlertTriangle size={14} /> {error}
            </p>
          )}
        </div>

        {/* Pie */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: '14px 24px', borderTop: '1px solid #f1f5f9', background: '#fafafa' }}>
          <span style={{ fontSize: 11, color: '#64748b' }}>
            {recipients.length === 0
              ? 'No hay destinatarios con correo.'
              : mode === 'access'
                ? (accessMode === 'invite'
                    ? <>Se enviará un <strong style={{ color: '#334155' }}>enlace de alta</strong> a {recipients.length} persona(s).</>
                    : <>Se generará una <strong style={{ color: '#334155' }}>contraseña nueva</strong> para {recipients.length} persona(s).</>)
              : item?.kind === 'survey'
                ? <>Se enviará la encuesta a <strong style={{ color: '#334155' }}>{recipients.length}</strong> persona(s).</>
                : <>Se enviará <strong style={{ color: '#334155' }}>1 correo personalizado</strong> a cada uno de los {recipients.length}.</>}
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {mode !== 'access' && (
              <TestSendButton size="sm" align="right" getPayload={testPayload} disabled={sending} />
            )}
            <button onClick={onClose} disabled={sending} style={{ padding: '9px 18px', borderRadius: 10, border: '1px solid #e2e8f0', background: '#fff', color: '#64748b', fontSize: 13, fontWeight: 600, cursor: 'pointer', fontFamily: 'inherit' }}>
              Cancelar
            </button>
            <button
              onClick={handleSend}
              disabled={!canSend}
              style={{
                display: 'flex', alignItems: 'center', gap: 7, padding: '9px 20px', borderRadius: 10, border: 'none',
                background: canSend ? '#7c3aed' : '#e2e8f0', color: canSend ? '#fff' : '#94a3b8',
                fontSize: 13, fontWeight: 700, cursor: canSend ? 'pointer' : 'not-allowed', fontFamily: 'inherit',
              }}
            >
              {sending
                ? <><Loader2 size={14} style={{ animation: 'obt-spin 0.9s linear infinite' }} /> Enviando...</>
                : <><Send size={14} /> Enviar a {recipients.length}</>}
            </button>
            <style>{'@keyframes obt-spin { to { transform: rotate(360deg) } }'}</style>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SendEmailModal;
