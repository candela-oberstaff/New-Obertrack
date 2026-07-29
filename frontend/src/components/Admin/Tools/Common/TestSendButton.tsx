import React, { useState, useEffect, useRef, useMemo } from 'react';
import { Beaker, Loader2, CheckCircle2, AlertTriangle, User as UserIcon } from 'lucide-react';
import { emailService, TestEmailPayload } from '../../../../services/emailService';
import { useAuth } from '../../../../context/AuthContext';
import { Select } from '../../../ui/Select';

/**
 * Envía una copia de prueba del correo a uno mismo antes del envío real.
 *
 * Hace falta porque la vista previa la compila el navegador, mientras que el
 * correo que sale de verdad lo compone el servidor y lo pasa por premailer: son
 * dos resultados parecidos pero no idénticos, y encima Gmail y Outlook recortan
 * CSS por su cuenta. Esto es lo único que enseña el resultado final.
 *
 * Con "Ver como" se eligen los datos de una persona real, así la prueba también
 * valida que sus campos existan y se lean bien, no solo que el diseño aguante.
 */

interface TestSendButtonProps {
  /** Se lee al pulsar, no al montar: el contenido cambia mientras se edita. */
  getPayload: () => Omit<TestEmailPayload, 'as_user_id'> | null;
  disabled?: boolean;
  size?: 'sm' | 'md';
  align?: 'left' | 'right';
}

type Status = { kind: 'idle' | 'sending' } | { kind: 'sent'; to: string; as: string } | { kind: 'error'; message: string };

export function TestSendButton({ getPayload, disabled, size = 'md', align = 'right' }: TestSendButtonProps) {
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [people, setPeople] = useState<{ id: number; name: string }[]>([]);
  const [asUserId, setAsUserId] = useState<number | ''>('');
  const [status, setStatus] = useState<Status>({ kind: 'idle' });
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open || people.length > 0) return;
    emailService.getAvailableRecipients()
      .then((res: any) => {
        const list = Array.isArray(res) ? res : (res?.data ?? res?.users ?? []);
        setPeople(list.filter((u: any) => u?.id && u?.name).map((u: any) => ({ id: u.id, name: u.name })));
      })
      .catch(() => { /* opcional: sin lista, la prueba usa datos de ejemplo */ });
  }, [open, people.length]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      const target = e.target as HTMLElement | null;
      if (wrapRef.current?.contains(target)) return;
      // El menú del Select se dibuja en un portal colgado de <body>, fuera de
      // este árbol. Sin esta excepción, elegir una opción se leía como un clic
      // fuera: el panel se cerraba y la selección se perdía por el camino.
      if (target?.closest?.('.ui-select__menu')) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const options = useMemo(
    () => people.map(p => ({ value: p.id, label: p.name })),
    [people]
  );

  const handleSend = async () => {
    const payload = getPayload();
    if (!payload) {
      setStatus({ kind: 'error', message: 'Todavía no hay contenido que probar.' });
      return;
    }
    setStatus({ kind: 'sending' });
    try {
      const res = await emailService.sendTestEmail({
        ...payload,
        as_user_id: asUserId === '' ? undefined : asUserId,
      });
      setStatus({ kind: 'sent', to: res.to, as: res.viewed_as });
    } catch (e: any) {
      const s = e?.response?.status;
      setStatus({
        kind: 'error',
        message: e?.response?.data?.error
          ?? (s === 404
            ? 'El servidor no reconoce esta acción (404). Puede que el backend no esté actualizado.'
            : s ? `El servidor respondió con un error ${s}.` : 'No hubo respuesta del servidor.'),
      });
    }
  };

  const compact = size === 'sm';
  const sending = status.kind === 'sending';

  return (
    <div ref={wrapRef} style={{ position: 'relative', flexShrink: 0 }}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => { setOpen(o => !o); setStatus({ kind: 'idle' }); }}
        title="Enviarte una copia de prueba antes del envío real"
        style={{
          display: 'flex', alignItems: 'center', gap: 6,
          padding: compact ? '6px 10px' : '8px 14px',
          borderRadius: 8,
          border: `1px solid ${open ? '#0891b2' : '#e2e8f0'}`,
          background: open ? '#ecfeff' : '#fff',
          color: disabled ? '#cbd5e1' : open ? '#0e7490' : '#64748b',
          fontSize: compact ? 11 : 12, fontWeight: 600,
          cursor: disabled ? 'not-allowed' : 'pointer',
          whiteSpace: 'nowrap', fontFamily: 'inherit', transition: 'all 0.15s',
        }}
      >
        <Beaker size={compact ? 12 : 14} style={{ color: disabled ? '#cbd5e1' : '#0891b2' }} />
        Enviar prueba
      </button>

      {open && (
        <div style={{
          position: 'absolute', top: 'calc(100% + 6px)', [align]: 0,
          width: 300, zIndex: 1200, background: '#fff',
          border: '1px solid #e2e8f0', borderRadius: 12,
          boxShadow: '0 12px 32px rgba(15,23,42,0.14)', padding: 14,
          display: 'flex', flexDirection: 'column', gap: 10,
        } as React.CSSProperties}>
          <div>
            <p style={{ margin: 0, fontSize: 11, color: '#64748b', lineHeight: 1.5 }}>
              Se enviará <strong style={{ color: '#334155' }}>solo a ti</strong>:
            </p>
            <p style={{ margin: '2px 0 0', fontSize: 12, fontWeight: 700, color: '#0e7490', wordBreak: 'break-all' }}>
              {user?.email ?? '—'}
            </p>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
            <label style={{ fontSize: 10, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              Ver como (opcional)
            </label>
            <Select
              fullWidth
              searchable
              clearable
              options={options}
              value={asUserId}
              onChange={v => setAsUserId(v === '' ? '' : Number(v))}
              placeholder="Datos de ejemplo"
              leftIcon={<UserIcon size={13} />}
              ariaLabel="Persona cuyos datos se usarán"
            />
            <span style={{ fontSize: 10, color: '#94a3b8', lineHeight: 1.5 }}>
              Elige a alguien real para comprobar que sus datos se ven bien. No recibirá nada.
            </span>
          </div>

          {status.kind === 'sent' && (
            <p style={{
              margin: 0, display: 'flex', alignItems: 'flex-start', gap: 6, fontSize: 11,
              color: '#15803d', background: '#f0fdf4', border: '1px solid #bbf7d0',
              borderRadius: 8, padding: '7px 9px', lineHeight: 1.5,
            }}>
              <CheckCircle2 size={13} style={{ flexShrink: 0, marginTop: 1 }} />
              <span>Prueba enviada a <strong>{status.to}</strong> con los datos de <strong>{status.as}</strong>. Llega con <strong>[PRUEBA]</strong> en el asunto.</span>
            </p>
          )}

          {status.kind === 'error' && (
            <p style={{
              margin: 0, display: 'flex', alignItems: 'flex-start', gap: 6, fontSize: 11,
              color: '#b91c1c', background: '#fef2f2', border: '1px solid #fecaca',
              borderRadius: 8, padding: '7px 9px', lineHeight: 1.5,
            }}>
              <AlertTriangle size={13} style={{ flexShrink: 0, marginTop: 1 }} /> {status.message}
            </p>
          )}

          <button
            type="button"
            onClick={handleSend}
            disabled={sending}
            style={{
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7,
              padding: '9px 14px', borderRadius: 9, border: 'none',
              background: sending ? '#cbd5e1' : '#0891b2', color: '#fff',
              fontSize: 12.5, fontWeight: 700, cursor: sending ? 'progress' : 'pointer',
              fontFamily: 'inherit',
            }}
          >
            {sending
              ? <><Loader2 size={13} style={{ animation: 'obt-test-spin 0.9s linear infinite' }} /> Enviando...</>
              : <><Beaker size={13} /> Enviarme la prueba</>}
          </button>
          <style>{'@keyframes obt-test-spin { to { transform: rotate(360deg) } }'}</style>
        </div>
      )}
    </div>
  );
}
