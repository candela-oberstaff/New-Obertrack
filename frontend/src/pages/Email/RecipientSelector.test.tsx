import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import RecipientSelector, { RecipientValue } from './RecipientSelector';
import { emailService } from '../../services/emailService';
import { audienceService } from '../../services/audienceService';

vi.mock('../../services/emailService', () => ({
  emailService: { getAvailableRecipients: vi.fn() },
}));

vi.mock('../../services/audienceService', () => ({
  audienceService: { getGroups: vi.fn() },
}));

const PEOPLE = [
  { id: 1, name: 'Ana Rivas', email: 'ana@x.com', user_type: 'profesional', is_manager: false, is_superadmin: false, is_active: true, country: 'Venezuela' },
  { id: 2, name: 'Beto Salas', email: 'beto@x.com', user_type: 'profesional', is_manager: false, is_superadmin: false, is_active: true, country: 'Venezuela' },
  { id: 3, name: 'Caro Díaz', email: 'caro@x.com', user_type: 'profesional', is_manager: false, is_superadmin: false, is_active: true, country: 'Colombia' },
  { id: 4, name: 'Dani Pérez', email: 'dani@x.com', user_type: 'empleador', is_manager: false, is_superadmin: false, is_active: true, country: 'Venezuela' },
  { id: 5, name: 'Eva Mora', email: 'eva@x.com', user_type: 'profesional', is_manager: false, is_superadmin: false, is_active: true, country: '' },
  { id: 6, name: 'Inactivo Juan', email: 'juan@x.com', user_type: 'profesional', is_manager: false, is_superadmin: false, is_active: false, country: 'Venezuela' },
];

const EMPTY: RecipientValue = { userIds: [], groupIds: [], expressContacts: [] };

const renderSelector = async (onChange = vi.fn()) => {
  render(<RecipientSelector value={EMPTY} onChange={onChange} />);
  await screen.findByText('Ana Rivas');
  return onChange;
};

const openCountries = () => fireEvent.click(screen.getByRole('button', { name: /todos los países/i }));

const pickCountry = async (label: string | RegExp) => {
  openCountries();
  const labelEl = await screen.findByText(label);
  fireEvent.click(labelEl);
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(emailService.getAvailableRecipients).mockResolvedValue({ data: PEOPLE } as never);
  vi.mocked(audienceService.getGroups).mockResolvedValue([] as never);
});

describe('RecipientSelector — filtro por rol', () => {
  it('separa profesionales activos de inactivos', async () => {
    await renderSelector();

    // Al seleccionar Profesionales activos
    fireEvent.click(screen.getByRole('button', { name: 'Profesionales activos' }));
    await waitFor(() => expect(screen.queryByText('Inactivo Juan')).not.toBeInTheDocument());
    expect(screen.getByText('Ana Rivas')).toBeInTheDocument();
    expect(screen.getByText('Beto Salas')).toBeInTheDocument();

    // Al seleccionar Profesionales inactivos
    fireEvent.click(screen.getByRole('button', { name: 'Profesionales inactivos' }));
    await waitFor(() => expect(screen.queryByText('Ana Rivas')).not.toBeInTheDocument());
    expect(screen.getByText('Inactivo Juan')).toBeInTheDocument();
  });

  it('combina rol activo y país', async () => {
    const onChange = await renderSelector();

    fireEvent.click(screen.getByRole('button', { name: 'Profesionales activos' }));
    openCountries();
    const opt = await screen.findByText('Venezuela (2)');
    fireEvent.click(opt);

    await waitFor(() => expect(screen.queryByText('Dani Pérez')).not.toBeInTheDocument());
    fireEvent.click(screen.getAllByRole('button', { name: 'Seleccionar todos' })[0]);

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ userIds: [1, 2] }));
  });
});

