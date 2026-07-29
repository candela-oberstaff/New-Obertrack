import { useState } from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConfirmProvider } from './ConfirmProvider'
import { Modal } from './Modal'

function Harness({ onClose, dirty = false }: { onClose: () => void; dirty?: boolean }) {
  return (
    <ConfirmProvider>
      <Modal isOpen onClose={onClose} title="Editar" isDirty={dirty}>
        <input aria-label="campo" />
      </Modal>
    </ConfirmProvider>
  )
}

const overlay = () => document.querySelector('.ui-modal__overlay') as HTMLElement

describe('Modal con isDirty', () => {
  it('sin cambios, el clic fuera cierra directo', () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} />)

    fireEvent.click(overlay())

    expect(onClose).toHaveBeenCalledTimes(1)
    expect(screen.queryByText('¿Descartar lo que hiciste?')).not.toBeInTheDocument()
  })

  it('con cambios, el clic fuera pide confirmación en vez de cerrar', async () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} dirty />)

    fireEvent.click(overlay())

    expect(await screen.findByText('¿Descartar lo que hiciste?')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('"Seguir editando" mantiene el diálogo abierto', async () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} dirty />)

    fireEvent.click(overlay())
    fireEvent.click(await screen.findByRole('button', { name: /seguir editando/i }))

    await waitFor(() => expect(screen.queryByText('¿Descartar lo que hiciste?')).not.toBeInTheDocument())
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByLabelText('campo')).toBeInTheDocument()
  })

  it('"Descartar y cerrar" sí cierra', async () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} dirty />)

    fireEvent.click(overlay())
    fireEvent.click(await screen.findByRole('button', { name: /descartar y cerrar/i }))

    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
  })

  // Escape y la X son igual de fáciles de pulsar sin querer que el clic fuera.
  it('Escape también pide confirmación', async () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} dirty />)

    fireEvent.keyDown(document, { key: 'Escape' })

    expect(await screen.findByText('¿Descartar lo que hiciste?')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('la X también pide confirmación', async () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} dirty />)

    fireEvent.click(screen.getByRole('button', { name: 'Cerrar' }))

    expect(await screen.findByText('¿Descartar lo que hiciste?')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  // El estado sucio se lee al cerrar, no al montar: un modal que empieza limpio
  // y se ensucia mientras escribes debe quedar protegido igual.
  it('protege lo escrito después de abrir', async () => {
    const onClose = vi.fn()

    function Live() {
      const [text, setText] = useState('')
      return (
        <ConfirmProvider>
          <Modal isOpen onClose={onClose} title="Editar" isDirty={text !== ''}>
            <input aria-label="campo" value={text} onChange={e => setText(e.target.value)} />
          </Modal>
        </ConfirmProvider>
      )
    }
    render(<Live />)

    fireEvent.change(screen.getByLabelText('campo'), { target: { value: 'hola' } })
    fireEvent.click(overlay())

    expect(await screen.findByText('¿Descartar lo que hiciste?')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })
})
