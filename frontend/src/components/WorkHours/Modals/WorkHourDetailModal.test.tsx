import { render } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { WorkHourDetailModal } from './WorkHourDetailModal'

/**
 * El modal empieza sin registro (workHour = null) y recibe uno al abrirse. Si
 * algún hook se llama por debajo del `return null` —o dentro del JSX, que es lo
 * mismo—, el número de hooks cambia entre ambos renders y React aborta con el
 * error #310, dejando la pantalla de horas en blanco al aprobar o rechazar.
 */
const registro = {
  id: 1,
  user_id: 1,
  work_date: '2026-07-30',
  work_type: 'complete',
  hours_worked: 8,
  approved: false,
  rejected: false,
} as never

const props = {
  onClose: vi.fn(),
  canApprove: true,
  canEdit: true,
  onApprove: vi.fn().mockResolvedValue(undefined),
  onReject: vi.fn().mockResolvedValue(undefined),
  onEdit: vi.fn(),
}

describe('WorkHourDetailModal', () => {
  it('no cambia el número de hooks al pasar de sin registro a con registro', () => {
    const { rerender } = render(<WorkHourDetailModal workHour={null as never} {...props} />)

    // Antes del arreglo esto lanzaba "Rendered more hooks than during the
    // previous render" y tumbaba toda la página.
    expect(() =>
      rerender(<WorkHourDetailModal workHour={registro} {...props} />)
    ).not.toThrow()
  })

  it('vuelve a cerrarse sin registro sin romper el render', () => {
    const { rerender } = render(<WorkHourDetailModal workHour={registro} {...props} />)
    expect(() =>
      rerender(<WorkHourDetailModal workHour={null as never} {...props} />)
    ).not.toThrow()
  })
})
