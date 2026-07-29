import { describe, it, expect } from 'vitest';
import { renderWithExamples, unknownVariables, insertAtCursor, variableToken, dropCaretOffset, highlightVariables } from './emailVariables';
import type { EmailVariable } from '../../../../services/emailService';

// Réplica del catálogo que sirve el backend (utils/email_vars.go).
const VARIABLES: EmailVariable[] = [
  { key: 'nombre', label: 'Nombre completo', description: '', example: 'María González', fallback: 'colega', group: 'Persona' },
  { key: 'primer_nombre', label: 'Primer nombre', description: '', example: 'María', fallback: 'colega', group: 'Persona' },
  { key: 'empresa', label: 'Empresa', description: '', example: 'Acme Corp', fallback: 'tu empresa', group: 'Empresa' },
  { key: 'pais', label: 'País', description: '', example: '', fallback: '', group: 'Ubicación' },
];

describe('renderWithExamples', () => {
  it('sustituye por el valor de ejemplo del catálogo', () => {
    expect(renderWithExamples('Hola {{primer_nombre}}', VARIABLES)).toBe('Hola María');
  });

  it('tolera espacios alrededor de la clave', () => {
    expect(renderWithExamples('{{ empresa }}', VARIABLES)).toBe('Acme Corp');
  });

  it('sin ejemplo cae al fallback del catálogo', () => {
    expect(renderWithExamples('Zona: {{pais}}', VARIABLES)).toBe('Zona: ');
  });

  it('el default en línea gana cuando no hay ejemplo', () => {
    expect(renderWithExamples('Zona: {{pais|LATAM}}', VARIABLES)).toBe('Zona: LATAM');
  });

  it('el ejemplo gana sobre el default en línea', () => {
    expect(renderWithExamples('{{empresa|tu empresa}}', VARIABLES)).toBe('Acme Corp');
  });

  it('nunca deja el token crudo de una clave desconocida', () => {
    expect(renderWithExamples('x{{no_existe}}y', VARIABLES)).toBe('xy');
  });

  it('deja intacto el texto sin variables', () => {
    expect(renderWithExamples('<p>Hola a todos</p>', VARIABLES)).toBe('<p>Hola a todos</p>');
  });
});

describe('unknownVariables', () => {
  it('detecta las claves que no existen en el catálogo', () => {
    expect(unknownVariables('{{nombre}} {{nombre_completo}}', VARIABLES)).toEqual(['nombre_completo']);
  });

  it('no repite la misma clave desconocida', () => {
    expect(unknownVariables('{{typo}} y otra vez {{typo}}', VARIABLES)).toEqual(['typo']);
  });

  it('devuelve vacío cuando todas existen', () => {
    expect(unknownVariables('{{nombre}} de {{empresa}}', VARIABLES)).toEqual([]);
  });
});

describe('insertAtCursor', () => {
  const field = (value: string, start: number, end = start) =>
    ({ value, selectionStart: start, selectionEnd: end }) as HTMLTextAreaElement;

  it('inserta en la posición del cursor', () => {
    const { value, caret } = insertAtCursor(field('Hola , ¿cómo estás?', 5), variableToken('nombre'));
    expect(value).toBe('Hola {{nombre}}, ¿cómo estás?');
    expect(caret).toBe(5 + '{{nombre}}'.length);
  });

  it('reemplaza el texto seleccionado', () => {
    const { value } = insertAtCursor(field('Hola AQUI!', 5, 9), variableToken('nombre'));
    expect(value).toBe('Hola {{nombre}}!');
  });

  it('anexa al final si no hay selección', () => {
    const { value } = insertAtCursor(field('Hola ', 5), variableToken('empresa'));
    expect(value).toBe('Hola {{empresa}}');
  });
});

// Sin caretRangeFromPoint (jsdom no lo implementa) el soltado debe caer al
// final del texto en vez de romperse o insertar en la posición cero.
describe('dropCaretOffset', () => {
  it('cae al final del texto cuando el navegador no resuelve el punto', () => {
    expect(dropCaretOffset(10, 10, 'Hola mundo')).toBe('Hola mundo'.length);
  });

  it('cae a 0 con texto vacío', () => {
    expect(dropCaretOffset(0, 0, '')).toBe(0);
  });
});

describe('highlightVariables', () => {
  const known = new Set(['nombre', 'empresa']);

  it('separa el texto de las variables', () => {
    expect(highlightVariables('Hola {{nombre}},', known)).toEqual([
      { text: 'Hola ', kind: 'text' },
      { text: '{{nombre}}', kind: 'known' },
      { text: ',', kind: 'text' },
    ]);
  });

  it('marca como desconocida una clave que no existe', () => {
    expect(highlightVariables('{{nombre_completo}}', known)).toEqual([
      { text: '{{nombre_completo}}', kind: 'unknown' },
    ]);
  });

  it('reconoce varias en la misma línea', () => {
    const segs = highlightVariables('{{nombre}} de {{empresa}}', known);
    expect(segs.filter(s => s.kind === 'known')).toHaveLength(2);
  });

  it('no pierde ni un carácter del código original', () => {
    const code = '<p>Hola <strong>{{nombre}}</strong>, de {{typo}}.</p>\n<!-- fin -->';
    expect(highlightVariables(code, known).map(s => s.text).join('')).toBe(code);
  });

  it('devuelve el código intacto cuando no hay variables', () => {
    expect(highlightVariables('<p>Sin variables</p>', known)).toEqual([
      { text: '<p>Sin variables</p>', kind: 'text' },
    ]);
  });

  it('trata el código vacío sin romperse', () => {
    expect(highlightVariables('', known)).toEqual([]);
  });

  // El regex del módulo es global y compartido: si highlightVariables avanzara
  // su lastIndex, la siguiente sustitución empezaría a mitad del texto.
  it('no deja el patrón compartido en un estado sucio', () => {
    const code = 'Hola {{nombre}}';
    highlightVariables(code, known);
    expect(renderWithExamples(code, VARIABLES)).toBe('Hola María González');
  });
});
