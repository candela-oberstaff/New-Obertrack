import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { TestSendButton } from './TestSendButton';
import { emailService } from '../../../../services/emailService';

vi.mock('../../../../services/emailService', () => ({
  emailService: {
    getAvailableRecipients: vi.fn(),
    sendTestEmail: vi.fn(),
  },
}));

vi.mock('../../../../context/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, name: 'Yo', email: 'yo@obertrack.com' } }),
}));

const PEOPLE = [
  { id: 10, name: 'Adriana Carolina Gómez Rondón' },
  { id: 11, name: 'Alejandra Zerpa Urbina' },
];

const payload = () => ({ subject: 'Hola {{nombre}}', blocks: '[{"type":"text"}]' });

const openPanel = async () => {
  fireEvent.click(screen.getByRole('button', { name: /enviar prueba/i }));
  await screen.findByText('yo@obertrack.com');
};

beforeEach(() => {
  // Sin esto las llamadas se acumulan entre casos y "no fue llamado" nunca pasa.
  vi.clearAllMocks();
  vi.mocked(emailService.getAvailableRecipients).mockResolvedValue(PEOPLE as never);
  vi.mocked(emailService.sendTestEmail).mockResolvedValue({ to: 'yo@obertrack.com', viewed_as: 'datos de ejemplo' });
});

describe('TestSendButton', () => {
  it('deja claro que la prueba solo llega a uno mismo', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    expect(screen.getByText('yo@obertrack.com')).toBeInTheDocument();
    // No hay campo para escribir otra dirección: es una decisión de seguridad.
    expect(screen.queryByPlaceholderText(/correo/i)).not.toBeInTheDocument();
  });

  // Regresión: el menú del Select se renderiza en un portal fuera del panel, y
  // el detector de "clic fuera" lo tomaba como salida. Se cerraba todo y no se
  // seleccionaba a nadie.
  it('elegir una persona no cierra el panel', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.click(await screen.findByRole('button', { name: /persona cuyos datos/i }));
    const option = await screen.findByText('Alejandra Zerpa Urbina');

    // El menú vive en un portal: hay que simular el mousedown que dispara el
    // cierre, no solo el click.
    fireEvent.mouseDown(option);
    fireEvent.click(option);

    expect(screen.getByText('yo@obertrack.com')).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /persona cuyos datos/i })).toHaveTextContent('Alejandra Zerpa Urbina')
    );
  });

  it('envía la prueba con los datos de la persona elegida', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.click(await screen.findByRole('button', { name: /persona cuyos datos/i }));
    const option = await screen.findByText('Adriana Carolina Gómez Rondón');
    fireEvent.mouseDown(option);
    fireEvent.click(option);

    fireEvent.click(screen.getByRole('button', { name: /enviarme la prueba/i }));

    await waitFor(() => expect(emailService.sendTestEmail).toHaveBeenCalledWith({
      subject: 'Hola {{nombre}}',
      blocks: '[{"type":"text"}]',
      as_user_id: 10,
    }));
  });

  it('sin elegir a nadie usa los datos de ejemplo', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.click(screen.getByRole('button', { name: /enviarme la prueba/i }));

    await waitFor(() => expect(emailService.sendTestEmail).toHaveBeenCalledWith({
      subject: 'Hola {{nombre}}',
      blocks: '[{"type":"text"}]',
      as_user_id: undefined,
    }));
  });

  it('avisa cuando todavía no hay nada que probar', async () => {
    render(<TestSendButton getPayload={() => null} />);
    await openPanel();

    fireEvent.click(screen.getByRole('button', { name: /enviarme la prueba/i }));

    expect(await screen.findByText(/no hay contenido que probar/i)).toBeInTheDocument();
    expect(emailService.sendTestEmail).not.toHaveBeenCalled();
  });

  it('un clic realmente fuera sí cierra el panel', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.mouseDown(document.body);

    await waitFor(() => expect(screen.queryByText('yo@obertrack.com')).not.toBeInTheDocument());
  });
});
