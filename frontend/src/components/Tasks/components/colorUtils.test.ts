import { BOARD_COLORS, hexToHsv, hsvToHex, isLightColor, normalizeHex } from './colorUtils'

// La conversión es lo único del selector que puede fallar sin que se note: un
// error de redondeo no rompe nada, solo guarda un color ligeramente distinto
// del que la persona vio.

describe('normalizeHex', () => {
  it('expande la forma corta y baja a minúsculas', () => {
    expect(normalizeHex('#ABC')).toBe('#aabbcc')
    expect(normalizeHex('FFF')).toBe('#ffffff')
    expect(normalizeHex('#10B981')).toBe('#10b981')
  })

  it('rechaza lo que no es un color', () => {
    expect(normalizeHex('')).toBeNull()
    expect(normalizeHex('#12')).toBeNull()
    expect(normalizeHex('#1234')).toBeNull()
    expect(normalizeHex('azul')).toBeNull()
    expect(normalizeHex('var(--primary)')).toBeNull()
  })
})

describe('hexToHsv / hsvToHex', () => {
  // Lo importante no es el HSV intermedio sino volver al mismo hex: es lo que
  // se guarda en el tablero.
  it('va y vuelve sin cambiar el color', () => {
    for (const color of BOARD_COLORS) {
      const hsv = hexToHsv(color)
      expect(hsv).not.toBeNull()
      expect(hsvToHex(hsv!)).toBe(color.toLowerCase())
    }
  })

  it('reconoce los tonos primarios', () => {
    expect(hexToHsv('#ff0000')).toMatchObject({ h: 0, s: 1, v: 1 })
    expect(hexToHsv('#00ff00')).toMatchObject({ h: 120, s: 1, v: 1 })
    expect(hexToHsv('#0000ff')).toMatchObject({ h: 240, s: 1, v: 1 })
  })

  it('trata el blanco y el negro como grises sin tono', () => {
    expect(hexToHsv('#ffffff')).toMatchObject({ s: 0, v: 1 })
    expect(hexToHsv('#000000')).toMatchObject({ s: 0, v: 0 })
  })

  it('devuelve null si el hex no sirve', () => {
    expect(hexToHsv('nope')).toBeNull()
  })
})

describe('isLightColor', () => {
  // Decide el color del tic sobre la muestra elegida.
  it('distingue los claros de los oscuros', () => {
    expect(isLightColor('#ffffff')).toBe(true)
    expect(isLightColor('#f59e0b')).toBe(true)
    expect(isLightColor('#060b23')).toBe(false)
    expect(isLightColor('#cc33cc')).toBe(false)
  })
})
