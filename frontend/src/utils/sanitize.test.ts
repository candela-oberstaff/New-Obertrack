import { describe, it, expect } from 'vitest'
import { htmlToText, htmlToLines } from './sanitize'

describe('htmlToText', () => {
  it('aplana el HTML del editor sin dejar markup a la vista', () => {
    const html = '<div>Lo que soporte puede hacer hoy y no podía ayer</div><div>Saber si el correo llegó</div>'
    expect(htmlToText(html)).toBe('Lo que soporte puede hacer hoy y no podía ayer Saber si el correo llegó')
  })

  it('separa los bloques: sin eso, la última palabra de uno se pega a la primera del siguiente', () => {
    expect(htmlToText('<div>ayer</div><div>Saber</div>')).toBe('ayer Saber')
    expect(htmlToText('<p>uno</p><p>dos</p>')).toBe('uno dos')
    expect(htmlToText('linea<br>otra')).toBe('linea otra')
  })

  it('respeta las mayúsculas interiores de una palabra', () => {
    // El parche que insertaba un espacio entre minúscula y mayúscula para
    // reparar las palabras pegadas rompía justo los nombres de producto.
    expect(htmlToText('<div>pérdida de mensajes en WhatsApp</div>')).toBe('pérdida de mensajes en WhatsApp')
  })

  it('decodifica las entidades', () => {
    expect(htmlToText('<div>Ancho&nbsp;&amp;&nbsp;alto &lt;3</div>')).toBe('Ancho & alto <3')
  })

  it('no deja pasar contenido activo', () => {
    expect(htmlToText('<img src=x onerror=alert(1)>hola')).toBe('hola')
    expect(htmlToText('<script>alert(1)</script>hola')).toBe('hola')
  })

  it('devuelve cadena vacía para lo vacío', () => {
    expect(htmlToText(undefined)).toBe('')
    expect(htmlToText(null)).toBe('')
    expect(htmlToText('<div><br></div>')).toBe('')
  })
})

describe('htmlToLines', () => {
  it('conserva los saltos de bloque', () => {
    expect(htmlToLines('<div>uno</div><div>dos</div>')).toBe('uno\ndos')
  })

  it('marca los elementos de lista', () => {
    expect(htmlToLines('<ul><li>uno</li><li>dos</li></ul>')).toBe('• uno\n• dos')
  })
})
