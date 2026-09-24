import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BoardEditModal } from './BoardEditModal'
import type { Board } from '../../../types'

// Editar un tablero era un prompt de texto, así que el color solo se podía
// elegir al crearlo y después quedaba congelado para siempre. Estas pruebas
// fijan lo que arregla eso: que el color se pueda cambiar desde aquí y que se
// mande junto al nombre.

const board = { id: 7, name: 'Marketing', color: '#3b82f6' } as Board

function abrir(onSubmit = vi.fn().mockResolvedValue(undefined)) {
  render(
    <BoardEditModal isOpen board={board} onClose={vi.fn()} onSubmit={onSubmit} />,
  )
  return onSubmit
}

describe('BoardEditModal', () => {
  it('llega con el nombre y el color del tablero ya puestos', () => {
    abrir()
    expect(screen.getByDisplayValue('Marketing')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Color #3b82f6' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('guarda el color nuevo sin tocar el nombre', async () => {
    const onSubmit = abrir()

    await userEvent.click(screen.getByRole('button', { name: 'Color #10b981' }))
    await userEvent.click(screen.getByRole('button', { name: 'Guardar' }))

    expect(onSubmit).toHaveBeenCalledWith({ name: 'Marketing', color: '#10b981' })
  })

  it('guarda el nombre nuevo conservando el color', async () => {
    const onSubmit = abrir()

    const input = screen.getByDisplayValue('Marketing')
    await userEvent.clear(input)
    await userEvent.type(input, 'Marketing LATAM')
    await userEvent.click(screen.getByRole('button', { name: 'Guardar' }))

    expect(onSubmit).toHaveBeenCalledWith({ name: 'Marketing LATAM', color: '#3b82f6' })
  })

  // Sin esto, abrir y cerrar el modal mandaba un guardado que reescribía el
  // tablero con lo mismo que ya tenía.
  it('no deja guardar si no se cambió nada', () => {
    abrir()
    expect(screen.getByRole('button', { name: 'Guardar' })).toBeDisabled()
  })

  it('no deja guardar con el nombre vacío', async () => {
    abrir()
    await userEvent.clear(screen.getByDisplayValue('Marketing'))
    expect(screen.getByRole('button', { name: 'Guardar' })).toBeDisabled()
  })
})
