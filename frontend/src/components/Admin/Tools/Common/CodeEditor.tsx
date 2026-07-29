import React, { useMemo, useRef } from 'react';
import { EmailVariable } from '../../../../services/emailService';
import { highlightVariables } from './emailVariables';

/**
 * Editor de código HTML con las variables resaltadas.
 *
 * Un <textarea> no admite texto de colores, así que se pintan dos capas: debajo
 * un <pre> con el mismo texto coloreado, y encima el textarea con el texto
 * transparente (solo se ve su cursor y su selección). Para que no se descuadren,
 * AMBAS capas comparten exactamente la misma tipografía, interlineado y relleno
 * —de ahí LAYER_STYLE— y el scroll se sincroniza.
 */

const KNOWN_COLOR = '#4ade80';   // verde: variable del catálogo
const UNKNOWN_COLOR = '#fbbf24'; // ámbar: no existe, se enviará vacía

// Todo lo que afecta a dónde cae cada carácter debe ser idéntico en las dos
// capas. Cualquier diferencia aquí desalinea el resaltado del texto real.
const LAYER_STYLE: React.CSSProperties = {
  margin: 0,
  padding: 16,
  border: 'none',
  fontFamily: 'monospace',
  fontSize: 12,
  lineHeight: 1.6,
  letterSpacing: 'normal',
  tabSize: 4,
  whiteSpace: 'pre-wrap',
  overflowWrap: 'break-word',
  wordBreak: 'normal',
};

interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  variables: EmailVariable[];
  placeholder?: string;
  /** Fondo del editor; el resto de colores se derivan de él. */
  background?: string;
  /** Props extra del textarea (enlace con el menú de variables, refs, ...). */
  textareaProps?: React.TextareaHTMLAttributes<HTMLTextAreaElement> & {
    ref?: (el: HTMLTextAreaElement | null) => void;
  };
}

export function CodeEditor({
  value, onChange, variables, placeholder, background = '#020617', textareaProps,
}: CodeEditorProps) {
  const backdropRef = useRef<HTMLPreElement>(null);

  const knownKeys = useMemo(
    () => new Set(variables.map(v => v.key)),
    [variables]
  );

  const segments = useMemo(
    () => highlightVariables(value, knownKeys),
    [value, knownKeys]
  );

  return (
    <div style={{ position: 'relative', flex: 1, minHeight: 0, background, overflow: 'hidden' }}>
      <pre
        ref={backdropRef}
        aria-hidden="true"
        style={{
          ...LAYER_STYLE,
          position: 'absolute',
          inset: 0,
          overflow: 'auto',
          color: '#e2e8f0',
          background: 'transparent',
          pointerEvents: 'none',
        }}
      >
        {segments.map((seg, i) =>
          seg.kind === 'text'
            ? seg.text
            : (
              <span
                key={i}
                style={{
                  color: seg.kind === 'known' ? KNOWN_COLOR : UNKNOWN_COLOR,
                  fontWeight: 700,
                  // Fondo tenue para localizarla de un vistazo sin desplazar
                  // ni un píxel el texto.
                  background: seg.kind === 'known'
                    ? 'rgba(74,222,128,0.14)'
                    : 'rgba(251,191,36,0.16)',
                  borderRadius: 3,
                }}
              >
                {seg.text}
              </span>
            )
        )}
        {/* Un textarea reserva una línea extra al final; el <pre> no. Sin esto
            las dos capas dejan de coincidir al escribir en la última línea. */}
        {'\n'}
      </pre>

      <textarea
        {...textareaProps}
        value={value}
        onChange={e => onChange(e.target.value)}
        onScroll={e => {
          const el = backdropRef.current;
          if (!el) return;
          el.scrollTop = e.currentTarget.scrollTop;
          el.scrollLeft = e.currentTarget.scrollLeft;
        }}
        spellCheck={false}
        placeholder={placeholder}
        style={{
          ...LAYER_STYLE,
          position: 'absolute',
          inset: 0,
          width: '100%',
          height: '100%',
          resize: 'none',
          outline: 'none',
          overflow: 'auto',
          background: 'transparent',
          // El texto se ve a través del textarea; lo que sí debe verse es el
          // cursor y la selección.
          color: 'transparent',
          caretColor: '#e2e8f0',
        }}
      />
    </div>
  );
}
