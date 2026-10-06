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

// El filtro de país es un multiselect propio: un botón ("Todos los países", o
// "N seleccionado(s)") abre un menú en un portal con buscador, "Seleccionar
// todos", "Limpiar" y una casilla por país. Marcar no cierra el menú; se cierra
// con un clic fuera.
const openCountries = () =>
  fireEvent.click(screen.getByRole('button', { name: /todos los países|seleccionados?$/i }));

const closeCountries = () => fireEvent.mouseDown(document.body);

// Las opciones del menú abierto, tal como se leen ("Venezuela (3)").
const countryOptions = () => {
  const menu = screen.getByPlaceholderText('Buscar...').closest('div')!.parentElement!;
  return Array.from(menu.querySelectorAll('label')).map(l => l.textContent);
};

const pickCountry = async (label: string | RegExp) => {
  openCountries();
  fireEvent.click(await screen.findByText(label));
  closeCountries();
};

const countryOption = async (label: RegExp) => {
  openCountries();
  const text = (await screen.findByText(label)).textContent;
  closeCountries();
  return text;
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
});

describe('RecipientSelector — filtro por país', () => {
  it('filtra el listado al país elegido', async () => {
    await renderSelector();

    await pickCountry(/^Venezuela \(/);

    await waitFor(() => expect(screen.queryByText('Caro Díaz')).not.toBeInTheDocument());
    expect(screen.getByText('Ana Rivas')).toBeInTheDocument();
    expect(screen.getByText('Beto Salas')).toBeInTheDocument();
  });

  it('los países se ordenan por cantidad y muestran cuántos hay', async () => {
    await renderSelector();

    openCountries();
    await screen.findByPlaceholderText('Buscar...');
    expect(countryOptions()).toEqual([
      'Venezuela (4)',
      'Colombia (1)',
      'Sin país registrado (1)',
    ]);
  });

  it('el conteo de países respeta el rol elegido', async () => {
    await renderSelector();

    fireEvent.click(screen.getByRole('button', { name: 'Profesionales activos' }));

    // Dani (empleador, Venezuela) e Inactivo Juan salen de la cuenta: quedan Ana y Beto.
    expect(await countryOption(/^Venezuela \(/)).toBe('Venezuela (2)');
  });

  it('permite encontrar a quienes no tienen país cargado', async () => {
    await renderSelector();

    await pickCountry(/^Sin país registrado/);

    await waitFor(() => expect(screen.getByText('Eva Mora')).toBeInTheDocument());
    expect(screen.queryByText('Ana Rivas')).not.toBeInTheDocument();
  });

  it('el país elegido se mantiene aunque el rol lo deje en cero', async () => {
    await renderSelector();

    await pickCountry(/^Colombia \(/);
    fireEvent.click(screen.getByRole('button', { name: 'Empresas' }));

    expect(await screen.findByText('Sin resultados para el filtro actual')).toBeInTheDocument();
    expect(screen.getByText('Colombia')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /1 seleccionado$/ })).toBeInTheDocument();
  });

  it('"Seleccionar todos" toma solo a los del país filtrado', async () => {
    const onChange = await renderSelector();

    await pickCountry(/^Venezuela \(/);
    await waitFor(() => expect(screen.queryByText('Caro Díaz')).not.toBeInTheDocument());
    fireEvent.click(screen.getAllByRole('button', { name: 'Seleccionar todos' })[0]);

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ userIds: [1, 2, 4, 6] }));
  });

  it('combina rol activo y país: profesionales de Venezuela', async () => {
    const onChange = await renderSelector();

    fireEvent.click(screen.getByRole('button', { name: 'Profesionales activos' }));
    await pickCountry(/^Venezuela \(/);
    await waitFor(() => expect(screen.queryByText('Dani Pérez')).not.toBeInTheDocument());
    fireEvent.click(screen.getAllByRole('button', { name: 'Seleccionar todos' })[0]);

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ userIds: [1, 2] }));
  });

  it('"Limpiar" vuelve a mostrar todos los países', async () => {
    await renderSelector();

    await pickCountry(/^Venezuela \(/);
    await waitFor(() => expect(screen.queryByText('Caro Díaz')).not.toBeInTheDocument());
    openCountries();
    fireEvent.click(await screen.findByRole('button', { name: 'Limpiar' }));
    closeCountries();

    await waitFor(() => expect(screen.getByText('Caro Díaz')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /todos los países/i })).toBeInTheDocument();
  });
});
