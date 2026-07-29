import { EmailVariable } from '../../../../services/emailService';

/**
 * Espejo en el cliente del motor de variables del backend
 * (backend/internal/utils/email_vars.go). Aquí solo se usa para PREVISUALIZAR:
 * la sustitución real, la que llega al destinatario, siempre ocurre en el
 * servidor al momento de enviar.
 *
 * Si cambias la sintaxis, cámbiala en los dos lados.
 */
export const EMAIL_VAR_REGEX = /\{\{\s*([a-zA-Z0-9_]+)\s*(\|([^{}]*))?\}\}/g;

/** Token tal cual se inserta en el editor. */
export const variableToken = (key: string) => `{{${key}}}`;

/**
 * Tipo MIME propio del arrastre de variables. Se transporta junto a text/plain
 * (que es lo que permite soltar sobre un input o textarea y que el navegador
 * inserte en el punto exacto del cursor), pero las zonas de soltado propias
 * solo aceptan este, para no reaccionar a cualquier texto arrastrado.
 */
export const VARIABLE_MIME = 'application/x-obertrack-variable';

/**
 * Posición dentro del texto donde se soltó el puntero. Sirve para insertar la
 * variable justo donde el usuario apuntó, y no siempre al final.
 *
 * Los bloques del lienzo renderizan su contenido como un único nodo de texto,
 * así que el offset del nodo coincide con el índice en `text`. Si se suelta
 * fuera del texto (sobre el padding, por ejemplo) se cae al final, que es el
 * comportamiento menos sorprendente.
 */
export function dropCaretOffset(x: number, y: number, text: string): number {
  const doc = document as Document & {
    caretRangeFromPoint?: (x: number, y: number) => Range | null;
    caretPositionFromPoint?: (x: number, y: number) => { offsetNode: Node; offset: number } | null;
  };

  let node: Node | null = null;
  let offset = 0;

  if (typeof doc.caretRangeFromPoint === 'function') {
    const range = doc.caretRangeFromPoint(x, y);
    if (range) {
      node = range.startContainer;
      offset = range.startOffset;
    }
  } else if (typeof doc.caretPositionFromPoint === 'function') {
    const pos = doc.caretPositionFromPoint(x, y);
    if (pos) {
      node = pos.offsetNode;
      offset = pos.offset;
    }
  }

  if (node?.nodeType === Node.TEXT_NODE) return Math.min(offset, text.length);
  return text.length;
}

/**
 * Sustituye los tokens por los valores de ejemplo del catálogo, replicando la
 * misma cascada del backend: valor → default en línea → fallback del catálogo
 * → vacío. Nunca deja el token crudo, igual que en el envío real.
 */
export function renderWithExamples(text: string, variables: EmailVariable[]): string {
  if (!text) return text;

  const byKey = new Map(variables.map(v => [v.key, v]));

  return text.replace(EMAIL_VAR_REGEX, (_match, rawKey: string, inlineGroup?: string, inlineDefault?: string) => {
    const variable = byKey.get(rawKey.toLowerCase());
    const value = variable?.example?.trim();
    if (value) return value;
    if (inlineGroup !== undefined) return (inlineDefault ?? '').trim();
    return variable?.fallback ?? '';
  });
}

export interface CodeSegment {
  text: string;
  /** 'known' = variable del catálogo, 'unknown' = escrita mal o inexistente. */
  kind: 'text' | 'known' | 'unknown';
}

/**
 * Trocea el código en fragmentos para poder pintar las variables de otro color
 * en el editor. Distingue las válidas de las que no existen, para que un error
 * de tecleo salte a la vista sin tener que enviar el correo.
 */
export function highlightVariables(code: string, knownKeys: Set<string>): CodeSegment[] {
  if (!code) return [];

  // Copia local del patrón: el regex del módulo es global y compartido, y
  // avanzar su lastIndex aquí rompería a quien lo use después.
  const re = new RegExp(EMAIL_VAR_REGEX.source, 'g');
  const segments: CodeSegment[] = [];
  let last = 0;

  for (let m = re.exec(code); m !== null; m = re.exec(code)) {
    if (m.index > last) segments.push({ text: code.slice(last, m.index), kind: 'text' });
    segments.push({
      text: m[0],
      kind: knownKeys.has(m[1].toLowerCase()) ? 'known' : 'unknown',
    });
    last = m.index + m[0].length;
  }

  if (last < code.length) segments.push({ text: code.slice(last), kind: 'text' });
  return segments;
}

/** Las claves usadas en el texto que no existen en el catálogo. */
export function unknownVariables(text: string, variables: EmailVariable[]): string[] {
  if (!text) return [];

  const known = new Set(variables.map(v => v.key));
  const found = new Set<string>();
  for (const match of text.matchAll(EMAIL_VAR_REGEX)) {
    const key = match[1].toLowerCase();
    if (!known.has(key)) found.add(key);
  }
  return [...found];
}

/**
 * Inserta un token en la posición del cursor de un campo de texto. Devuelve el
 * valor nuevo y dónde debe quedar el cursor; aplicar el estado es tarea de
 * quien llama, porque el valor lo controla React.
 */
export function insertAtCursor(
  field: HTMLInputElement | HTMLTextAreaElement,
  token: string
): { value: string; caret: number } {
  const current = field.value ?? '';
  const start = field.selectionStart ?? current.length;
  const end = field.selectionEnd ?? start;

  return {
    value: current.slice(0, start) + token + current.slice(end),
    caret: start + token.length,
  };
}
