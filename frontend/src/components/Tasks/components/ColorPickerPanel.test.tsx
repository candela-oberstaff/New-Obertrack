import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fireEvent } from '@testing-library/react'
import { ColorPickerPanel } from './ColorPickerPanel'

// Este panel existe para no abrir la ventana de color del sistema operativo.
// Lo que hay que sostener es que las tres formas de elegir —tono, teclado sobre
// el cuadro y hex escrito— acaban en el mismo sitio: un color válido hacia
// arriba.

describe('ColorPickerPanel', () => {
  it('avisa del color nuevo al mover el tono', () => {
    const onChange = vi.fn()
    render(<ColorPickerPanel value="#ff0000" onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Tono'), { target: { value: '120' } })

    expect(onChange).toHaveBeenCalledWith('#00ff00')
  })

  it('acepta un hex escrito a mano', async () => {
    const onChange = vi.fn()
    render(<ColorPickerPanel value="#ff0000" onChange={onChange} />)

    const hex = screen.getByLabelText('Código hexadecimal del color')
    await userEvent.clear(hex)
    await userEvent.paste('#10b981')

    expect(onChange).toHaveBeenLastCalledWith('#10b981')
  })

  // Escribir es teclear de a poco: '#1', '#10'... Ninguno de esos pasos es un
  // color, y si se propagaran el tablero cambiaría de color a cada tecla.
  it('ignora lo que todavía no es un color', async () => {
    const onChange = vi.fn()
    render(<ColorPickerPanel value="#ff0000" onChange={onChange} />)

    const hex = screen.getByLabelText('Código hexadecimal del color')
    await userEvent.clear(hex)
    await userEvent.type(hex, '#10b9')

    expect(onChange).not.toHaveBeenCalled()
  })

  // El cuadro de saturación y brillo se usa arrastrando, pero tiene que poder
  // manejarse con el teclado: arrastrar no es una opción para todo el mundo.
  it('se maneja con las flechas', () => {
    const onChange = vi.fn()
    render(<ColorPickerPanel value="#ff0000" onChange={onChange} />)

    const area = screen.getByLabelText('Saturación y brillo')
    fireEvent.keyDown(area, { key: 'ArrowLeft' })

    expect(onChange).toHaveBeenCalledTimes(1)
    // Menos saturación sobre rojo puro aclara hacia el blanco.
    expect(onChange.mock.calls[0][0]).not.toBe('#ff0000')
  })

  it('anuncia la saturación y el brillo para quien no ve el cuadro', () => {
    render(<ColorPickerPanel value="#ff0000" onChange={vi.fn()} />)

    expect(screen.getByLabelText('Saturación y brillo')).toHaveAttribute(
      'aria-valuetext',
      'Saturación 100%, brillo 100%',
    )
  })
})
