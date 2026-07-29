import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { Braces, Search, AlertTriangle, Check, GripVertical } from 'lucide-react';
import { emailService, EmailVariable } from '../../../../services/emailService';
import { insertAtCursor, variableToken, unknownVariables, VARIABLE_MIME, dropCaretOffset } from './emailVariables';

/**
 * Menú de variables de personalización, compartido por TODOS los editores de
 * correo (campañas y plantillas) y por sus modos Visual y HTML. Los tokens que
 * inserta son los mismos en los cuatro casos porque el backend los resuelve
 * sobre el HTML ya compilado, sin importar de dónde salió.
 */

/**
 * Vincula un campo de texto al menú: al enfocarlo pasa a ser el destino donde
 * se insertan los tokens. `apply` escribe el valor nuevo en el estado de React
 * que controla ese campo.
 *
 * Con `{ isDefault: true }` el campo queda como destino desde que se monta, sin
 * necesidad de enfocarlo. Se usa en el editor de código, donde solo hay un
 * campo posible y esperar un clic previo sería un paso de más.
 */
export type FieldBinder = (
  apply: (value: string) => void,
  label: string,
  opts?: { isDefault?: boolean }
) => {
  onFocus: (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  onBlur: (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  ref?: (el: HTMLInputElement | HTMLTextAreaElement | null) => void;
};

type FieldTarget = {
  el: HTMLInputElement | HTMLTextAreaElement;
  apply: (v: string) => void;
  label: string;
  /**
   * Última posición del cursor conocida. Abrir el menú o hacer clic en una
   * variable le quita el foco al campo, y no todos los navegadores conservan
   * selectionStart tras el blur; anotarlo garantiza insertar donde estaba.
   */
  caret: { start: number; end: number } | null;
};

function readCaret(el: HTMLInputElement | HTMLTextAreaElement) {
  const start = el.selectionStart ?? el.value.length;
  return { start, end: el.selectionEnd ?? start };
}

/**
 * Carga el catálogo de variables y gestiona en qué campo se inserta el token.
 * El catálogo lo define el backend (utils/email_vars.go): es la misma lista que
 * usa al enviar, así el editor nunca ofrece un token que no se vaya a resolver.
 */
export function useEmailVariables() {
  const [variables, setVariables] = useState<EmailVariable[]>([]);
  const [activeFieldLabel, setActiveFieldLabel] = useState<string | null>(null);
  const activeField = useRef<FieldTarget | null>(null);
  const defaultField = useRef<FieldTarget | null>(null);

  useEffect(() => {
    emailService.getVariables()
      .then(setVariables)
      .catch(err => console.error('Error fetching email variables:', err));
  }, []);

  // Al cambiar de pestaña (Visual ↔ HTML) o de paso, el campo enfocado sale
  // del DOM y deja de ser un destino válido. Sin deps a propósito: la
  // comprobación corre tras cada render, ya con el DOM actualizado.
  useEffect(() => {
    if (activeField.current && !activeField.current.el.isConnected) {
      activeField.current = null;
      setActiveFieldLabel(null);
    }
  });

  const bindField = useCallback<FieldBinder>((apply, label, opts) => {
    const binding = {
      onFocus: (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        activeField.current = { el: e.currentTarget, apply, label, caret: readCaret(e.currentTarget) };
        setActiveFieldLabel(label);
      },
      onBlur: (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        if (activeField.current?.el === e.currentTarget) {
          activeField.current.caret = readCaret(e.currentTarget);
        }
      },
    };

    if (!opts?.isDefault) return binding;

    return {
      ...binding,
      ref: (el: HTMLInputElement | HTMLTextAreaElement | null) => {
        defaultField.current = el ? { el, apply, label, caret: null } : null;
      },
    };
  }, []);

  const insertVariable = useCallback((key: string) => {
    // Un campo desmontado escribiría sobre un editor que ya no está a la vista.
    const target = [activeField.current, defaultField.current]
      .find(t => t?.el.isConnected);
    if (!target) return false;

    // El campo ya perdió el foco (el menú se lo llevó): se le devuelve el
    // cursor a donde estaba antes de leer la posición de inserción.
    if (target.caret && document.activeElement !== target.el) {
      target.el.setSelectionRange(target.caret.start, target.caret.end);
    }

    const { value, caret } = insertAtCursor(target.el, variableToken(key));
    target.apply(value);

    const { el } = target;
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(caret, caret);
    });
    return true;
  }, []);

  /** Claves usadas en el texto que no existen en el catálogo. */
  const findUnknown = useCallback(
    (text: string) => unknownVariables(text, variables),
    [variables]
  );

  // ── Arrastrar y soltar ────────────────────────────────────────────────────
  // Soltar sobre un input o textarea lo resuelve el navegador con el text/plain
  // del arrastre, insertando en el punto exacto del cursor. Lo que sí hay que
  // manejar a mano es soltar sobre un bloque del lienzo, que no es un campo.

  const [dragging, setDragging] = useState(false);

  const dragSourceProps = useCallback((key: string) => ({
    draggable: true,
    onDragStart: (e: React.DragEvent) => {
      const token = variableToken(key);
      e.dataTransfer.setData('text/plain', token);
      e.dataTransfer.setData(VARIABLE_MIME, token);
      e.dataTransfer.effectAllowed = 'copy';
      setDragging(true);
    },
    onDragEnd: () => setDragging(false),
  }), []);

  /**
   * Convierte un elemento en zona de soltado para variables. `text` es el
   * contenido actual y `apply` lo reemplaza por el que resulte de insertar.
   */
  const dropZone = useCallback((text: string, apply: (value: string) => void) => ({
    onDragOver: (e: React.DragEvent) => {
      if (!e.dataTransfer.types.includes(VARIABLE_MIME)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = 'copy';
    },
    onDrop: (e: React.DragEvent) => {
      const token = e.dataTransfer.getData(VARIABLE_MIME);
      if (!token) return;
      e.preventDefault();
      // Sin esto el contenedor padre volvería a insertar el mismo token.
      e.stopPropagation();
      setDragging(false);

      const at = dropCaretOffset(e.clientX, e.clientY, text);
      apply(text.slice(0, at) + token + text.slice(at));
    },
  }), []);

  return {
    variables, bindField, insertVariable, activeFieldLabel, findUnknown,
    dragging, dragSourceProps, dropZone,
  };
}

interface VariableMenuProps {
  variables: EmailVariable[];
  /** Devuelve false si no había un campo donde insertar. */
  onInsert: (key: string) => boolean;
  activeFieldLabel: string | null;
  /** Props de arrastre por variable, provistas por useEmailVariables. */
  dragSourceProps: (key: string) => React.HTMLAttributes<HTMLElement> & { draggable: boolean };
  /** Variables escritas en el contenido que no existen en el catálogo. */
  unknown?: string[];
  size?: 'sm' | 'md';
  /** Lado por el que se despliega el panel respecto al botón. */
  align?: 'left' | 'right';
}

export function VariableMenu({
  variables, onInsert, activeFieldLabel, dragSourceProps, unknown = [], size = 'md', align = 'right',
}: VariableMenuProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [justInserted, setJustInserted] = useState<string | null>(null);
  const [noTarget, setNoTarget] = useState(false);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);

  // Se cierra al hacer clic fuera o con Escape, como cualquier menú.
  useEffect(() => {
    if (!open) return;

    const onPointerDown = (e: MouseEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };

    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const groups = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const matches = needle
      ? variables.filter(v =>
          v.label.toLowerCase().includes(needle) ||
          v.key.includes(needle) ||
          v.description.toLowerCase().includes(needle))
      : variables;

    return matches.reduce<Record<string, EmailVariable[]>>((acc, v) => {
      (acc[v.group] ||= []).push(v);
      return acc;
    }, {});
  }, [variables, query]);

  if (variables.length === 0) return null;

  const handleInsert = (key: string) => {
    if (!onInsert(key)) {
      // Nadie ha elegido dónde insertar: sin aviso, el clic no haría nada y
      // parecería que el menú está roto.
      setNoTarget(true);
      window.setTimeout(() => setNoTarget(false), 2500);
      return;
    }
    // El panel sigue abierto para poder insertar varias seguidas.
    setJustInserted(key);
    window.setTimeout(() => setJustInserted(k => (k === key ? null : k)), 900);
  };

  const compact = size === 'sm';

  return (
    <div ref={wrapRef} style={{ position: 'relative', flexShrink: 0 }}>
      <button
        type="button"
        // Evita que el campo de texto pierda el foco al abrir el menú: el token
        // debe insertarse donde estaba el cursor.
        onMouseDown={e => e.preventDefault()}
        onClick={() => {
          setOpen(o => !o);
          setQuery('');
        }}
        title="Insertar una variable de personalización"
        style={{
          display: 'flex', alignItems: 'center', gap: 6,
          padding: compact ? '6px 10px' : '8px 14px',
          borderRadius: 8,
          border: `1px solid ${open ? '#7c3aed' : '#e2e8f0'}`,
          background: open ? '#f5f3ff' : '#fff',
          color: open ? '#6d28d9' : '#64748b',
          fontSize: compact ? 11 : 12, fontWeight: 600, cursor: 'pointer',
          whiteSpace: 'nowrap', fontFamily: 'inherit', transition: 'all 0.15s',
        }}
      >
        <Braces size={compact ? 12 : 14} style={{ color: '#7c3aed' }} />
        Variables
        {unknown.length > 0 && (
          <span
            title={`${unknown.length} variable(s) sin definir en el contenido`}
            style={{
              width: 7, height: 7, borderRadius: '50%', background: '#f59e0b',
              display: 'inline-block', marginLeft: 2,
            }}
          />
        )}
      </button>

      {open && (
        <div style={{
          position: 'absolute', top: 'calc(100% + 6px)', [align]: 0,
          width: 300, maxHeight: 420, zIndex: 1200,
          background: '#fff', border: '1px solid #e2e8f0', borderRadius: 12,
          boxShadow: '0 12px 32px rgba(15,23,42,0.14)',
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        } as React.CSSProperties}>
          {/*
            El hover de las filas va por CSS y no por handlers que toquen
            element.style: cualquier mutación del DOM sobre el elemento que se
            está arrastrando (o sus ancestros) hace que Chrome cancele el
            arrastre nada más empezar.
          */}
          <style>{`
            .obt-var-row { cursor: grab; }
            .obt-var-row:hover { background: #f5f3ff; }
            .obt-var-row:active { cursor: grabbing; }
          `}</style>
          {/* Buscador */}
          <div style={{ padding: 10, borderBottom: '1px solid #f1f5f9', flexShrink: 0 }}>
            <div style={{ position: 'relative' }}>
              <Search size={13} style={{ position: 'absolute', left: 9, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8' }} />
              <input
                ref={searchRef}
                autoFocus
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder="Buscar variable..."
                style={{
                  width: '100%', padding: '7px 10px 7px 28px', borderRadius: 8,
                  border: '1px solid #e2e8f0', background: '#f8fafc', fontSize: 12,
                  outline: 'none', color: '#1e293b', fontFamily: 'inherit',
                }}
              />
            </div>
          </div>

          {/* Listado */}
          <div style={{ overflowY: 'auto', flex: 1, padding: '4px 0' }}>
            {Object.keys(groups).length === 0 && (
              <p style={{ margin: 0, padding: '18px 14px', fontSize: 12, color: '#94a3b8', textAlign: 'center' }}>
                Sin coincidencias
              </p>
            )}

            {Object.entries(groups).map(([group, vars]) => (
              <div key={group}>
                <div style={{
                  padding: '8px 14px 4px', fontSize: 9, fontWeight: 700, color: '#94a3b8',
                  textTransform: 'uppercase', letterSpacing: '0.09em',
                }}>
                  {group}
                </div>
                {vars.map(v => {
                  const drag = dragSourceProps(v.key);
                  return (
                    <div
                      key={v.key}
                      {...drag}
                      className="obt-var-row"
                      role="button"
                      tabIndex={0}
                      // Sin onMouseDown+preventDefault a propósito: bloquear la
                      // acción por defecto del mousedown impide que arranque el
                      // arrastre nativo. El cursor del campo se conserva aparte,
                      // anotándolo al perder el foco (ver bindField).
                      onClick={() => handleInsert(v.key)}
                      onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleInsert(v.key); } }}
                      onDragEnd={e => {
                        drag.onDragEnd?.(e);
                        // dropEffect es 'none' cuando el arrastre se canceló o
                        // se soltó en el vacío: ahí el panel debe seguir
                        // abierto para reintentar. Solo se cierra tras un
                        // soltado real, y aplazado, para no tocar el DOM del
                        // origen mientras el navegador cierra la operación.
                        if (e.dataTransfer.dropEffect !== 'none') {
                          window.setTimeout(() => setOpen(false), 0);
                        }
                      }}
                      title={v.fallback
                        ? `${v.description}. Si el destinatario no tiene el dato se escribe "${v.fallback}".`
                        : v.description}
                      style={{
                        display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8,
                        padding: '7px 14px 7px 8px', textAlign: 'left', fontFamily: 'inherit',
                      }}
                    >
                      <GripVertical size={13} style={{ color: '#cbd5e1', flexShrink: 0 }} />
                      <span style={{ display: 'flex', flexDirection: 'column', gap: 1, minWidth: 0, flex: 1 }}>
                        <span style={{ fontSize: 12, fontWeight: 600, color: '#1e293b' }}>{v.label}</span>
                        <span style={{
                          fontSize: 10, color: '#94a3b8', overflow: 'hidden',
                          textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                        }}>
                          {v.example}
                        </span>
                      </span>
                      {justInserted === v.key ? (
                        <span style={{ display: 'flex', alignItems: 'center', gap: 3, fontSize: 10, fontWeight: 700, color: '#16a34a', flexShrink: 0 }}>
                          <Check size={11} /> Listo
                        </span>
                      ) : (
                        <code style={{
                          fontSize: 10, color: '#7c3aed', background: '#f5f3ff', border: '1px solid #ede9fe',
                          borderRadius: 5, padding: '2px 5px', flexShrink: 0, fontFamily: 'monospace',
                        }}>
                          {variableToken(v.key)}
                        </code>
                      )}
                    </div>
                  );
                })}
              </div>
            ))}
          </div>

          {/* Pie: dónde se inserta y avisos */}
          <div style={{ borderTop: '1px solid #f1f5f9', padding: '8px 14px', flexShrink: 0, background: '#fafafa' }}>
            {unknown.length > 0 && (
              <div style={{
                display: 'flex', alignItems: 'flex-start', gap: 6, marginBottom: 6, fontSize: 10,
                color: '#b45309', background: '#fffbeb', border: '1px solid #fde68a',
                borderRadius: 6, padding: '5px 7px',
              }}>
                <AlertTriangle size={12} style={{ flexShrink: 0, marginTop: 1 }} />
                <span>
                  Sin definir: {unknown.map(k => `{{${k}}}`).join(' ')} — se enviarán vacías.
                </span>
              </div>
            )}
            {noTarget ? (
              <p style={{
                margin: 0, fontSize: 10, color: '#b45309', background: '#fffbeb',
                border: '1px solid #fde68a', borderRadius: 6, padding: '5px 7px', lineHeight: 1.5,
              }}>
                Primero haz clic en el texto donde quieres la variable, o arrástrala hasta ahí.
              </p>
            ) : (
              <p style={{ margin: 0, fontSize: 10, color: '#94a3b8', lineHeight: 1.5 }}>
                <strong style={{ color: '#64748b' }}>Arrastra</strong> una variable al correo, o haz clic para insertarla{' '}
                {activeFieldLabel
                  ? <>en <strong style={{ color: '#64748b' }}>{activeFieldLabel}</strong>.</>
                  : <>tras elegir un campo de texto.</>}
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
