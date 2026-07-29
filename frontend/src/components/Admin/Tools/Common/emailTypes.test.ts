import { describe, it, expect } from 'vitest';
import { compileBlocksToHTML, templateToHTML, blocksFromJSON, type EmailBlock } from './emailTypes';

/**
 * Una plantilla escrita a mano se guarda como un único bloque marcado `raw`:
 * su contenido ya ES el correo completo. Recompilarlo lo mete dentro del marco
 * del editor visual y lo destruye.
 */
const RAW_EMAIL = `<!DOCTYPE html>
<html><head><style>
  @media only screen and (max-width: 600px) { .content-padding { padding: 20px !important; } }
</style></head>
<body>
  <table><tr><td class="content-padding">
    <p>Hola <strong>{{nombre}}</strong>,</p>
  </td></tr></table>
</body></html>`;

const rawBlocks = (): EmailBlock[] => [
  { id: 'raw_code', type: 'text', content: RAW_EMAIL, style: { raw: 'true' } },
];

describe('templateToHTML', () => {
  it('devuelve intacta una plantilla escrita a mano', () => {
    expect(templateToHTML(rawBlocks())).toBe(RAW_EMAIL);
  });

  it('sobrevive a guardar y volver a cargar', () => {
    const json = JSON.stringify([{ type: 'text', content: RAW_EMAIL, style: { raw: 'true' } }]);
    expect(templateToHTML(blocksFromJSON(json))).toBe(RAW_EMAIL);
  });

  it('compila normalmente los diseños hechos con bloques', () => {
    const blocks: EmailBlock[] = [
      { id: '1', type: 'text', content: 'Hola', style: {} },
    ];
    const html = templateToHTML(blocks);
    expect(html).toContain('Hola');
    expect(html).toContain('max-width:600px');
  });
});

// Regresión: al pasar a la pestaña HTML se recompilaba SIEMPRE, así que una
// plantilla cruda quedaba envuelta en el marco del editor. El primer carácter
// que se escribiera guardaba esa versión envuelta y el preview reventaba: el
// documento entero acababa dentro de un div con white-space:pre-wrap, que
// convierte cada salto de línea del código en espacio en blanco visible.
describe('compileBlocksToHTML sobre una plantilla cruda (lo que NO hay que hacer)', () => {
  const wrapped = compileBlocksToHTML(rawBlocks());

  it('la envuelve en el marco del editor', () => {
    expect(wrapped).not.toBe(RAW_EMAIL);
    expect(wrapped).toContain('logo-oberstaff.png');
  });

  it('mete el documento en un contenedor con white-space:pre-wrap', () => {
    expect(wrapped).toContain('white-space:pre-wrap');
  });

  it('duplica la estructura del documento', () => {
    expect(wrapped.match(/<!DOCTYPE html>/gi)?.length).toBe(1);
    // El <html> del usuario queda anidado dentro del div contenedor del editor.
    expect(wrapped.indexOf('<div style="background-color:#ffffff')).toBeLessThan(
      wrapped.indexOf('<!DOCTYPE html>')
    );
  });
});
