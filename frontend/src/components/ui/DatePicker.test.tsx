import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DatePicker, fromISODate, toISODate } from './DatePicker'

/** Envoltorio controlado: el componente no guarda el valor, igual que en la app. */
function Harness({ initial = '', ...rest }: { initial?: string } & Partial<React.ComponentProps<typeof DatePicker>>) {
  const [v, setV] = useState(initial)
  return (
    <>
      <DatePicker value={v} onChange={setV} ariaLabel="Fecha" {...rest} />
      <span data-testid="valor">{v}</span>
    </>
  )
}

const field = () => screen.getByLabelText('Fecha')
const valor = () => screen.getByTestId('valor').textContent

describe('DatePicker', () => {
  it('muestra el valor en dd/mm/aaaa y no en el ISO que habla el backend', () => {
    render(<Harness initial="2021-03-15" />)
    expect(field()).toHaveValue('15/03/2021')
  })

  it('acepta la fecha tecleada y la emite en ISO', async () => {
    render(<Harness />)
    await userEvent.type(field(), '15/03/2021')
    await userEvent.tab()
    expect(valor()).toBe('2021-03-15')
  })

  // Escribir es la vía rápida para quien carga fechas a diario; si al salir del
  // campo se perdiera lo tecleado, el componente sería peor que el nativo.
  it('acepta también el ISO tecleado y descarta lo que no es una fecha', async () => {
    render(<Harness initial="2021-03-15" />)
    await userEvent.clear(field())
    await userEvent.type(field(), '2020-01-02')
    await userEvent.tab()
    expect(valor()).toBe('2020-01-02')

    await userEvent.clear(field())
    await userEvent.type(field(), 'el jueves')
    await userEvent.tab()
    // Se queda la última válida, y el campo vuelve a enseñarla.
    expect(valor()).toBe('2020-01-02')
    expect(field()).toHaveValue('02/01/2020')
  })

  it('rechaza un día que no existe en vez de desbordar al mes siguiente', async () => {
    render(<Harness initial="2021-03-15" />)
    await userEvent.clear(field())
    await userEvent.type(field(), '31/02/2021')
    await userEvent.tab()
    expect(valor()).toBe('2021-03-15')
  })

  it('elige un día desde el calendario', async () => {
    render(<Harness initial="2021-03-15" />)
    await userEvent.click(screen.getByLabelText('Abrir calendario'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('Marzo de 2021')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: '20 de marzo de 2021' }))
    expect(valor()).toBe('2021-03-20')
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('navega por mes y por año', async () => {
    render(<Harness initial="2021-03-15" />)
    await userEvent.click(screen.getByLabelText('Abrir calendario'))

    await userEvent.click(screen.getByLabelText('Mes anterior'))
    expect(screen.getByText('Febrero de 2021')).toBeInTheDocument()

    await userEvent.click(screen.getByLabelText('Año anterior'))
    expect(screen.getByText('Febrero de 2020')).toBeInTheDocument()

    await userEvent.click(screen.getByLabelText('Año siguiente'))
    await userEvent.click(screen.getByLabelText('Mes siguiente'))
    expect(screen.getByText('Marzo de 2021')).toBeInTheDocument()
  })

  // El caso de la fecha de ingreso: max="hoy" tiene que cortar el futuro tanto
  // en el calendario como al teclear, o el 400 del backend llega por sorpresa.
  it('respeta min y max al hacer clic y al teclear', async () => {
    render(<Harness initial="2021-03-15" min="2021-03-10" max="2021-03-20" />)
    await userEvent.click(screen.getByLabelText('Abrir calendario'))

    expect(screen.getByRole('button', { name: '9 de marzo de 2021' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '21 de marzo de 2021' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '12 de marzo de 2021' })).not.toBeDisabled()

    await userEvent.keyboard('{Escape}')
    await userEvent.clear(field())
    await userEvent.type(field(), '25/03/2021')
    await userEvent.tab()
    expect(valor()).toBe('2021-03-15')
  })

  it('borra el valor solo cuando se permite', async () => {
    render(<Harness initial="2021-03-15" clearable />)
    await userEvent.click(screen.getByLabelText('Abrir calendario'))
    await userEvent.click(screen.getByRole('button', { name: 'Borrar' }))
    expect(valor()).toBe('')

    // Vaciar el campo a mano también limpia: es lo que hacía el nativo.
    render(<Harness initial="2021-03-15" />)
    const second = screen.getAllByLabelText('Fecha')[1]
    await userEvent.clear(second)
    await userEvent.tab()
    expect(screen.getAllByTestId('valor')[1].textContent).toBe('')
  })

  it('cierra con Escape sin tocar el valor', async () => {
    render(<Harness initial="2021-03-15" />)
    await userEvent.click(screen.getByLabelText('Abrir calendario'))
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(valor()).toBe('2021-03-15')
  })
})

// Al convertir con toISOString() la fecha se pasa a UTC y en cualquier huso al
// oeste de Greenwich devuelve el día anterior. Es el fallo clásico de un
// calendario y aquí se traduciría en fechas de ingreso corridas un día.
describe('conversión de fechas', () => {
  it('no corre el día al ir y volver de ISO', () => {
    expect(toISODate(new Date(2021, 2, 15))).toBe('2021-03-15')
    expect(toISODate(new Date(2021, 0, 1))).toBe('2021-01-01')
    const d = fromISODate('2021-01-01')!
    expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2021, 0, 1])
  })

  it('rechaza lo que no es una fecha real', () => {
    expect(fromISODate('2021-02-31')).toBeNull()
    expect(fromISODate('')).toBeNull()
    expect(fromISODate(null)).toBeNull()
  })
})
