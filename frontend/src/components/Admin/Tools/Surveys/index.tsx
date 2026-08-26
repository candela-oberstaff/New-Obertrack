import React, { useState, useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ClipboardList, Plus } from 'lucide-react';
import listStyles from './SurveyList.module.css';
import commonStyles from '../Tools.module.css';
import SurveyBuilder from './SurveyBuilder';
import SurveyResults from './SurveyResults';
import SurveyCard from './components/SurveyCard';
import { surveyService, Survey } from '../../../../services/surveyService';
import { useConfirm } from '../../../ui/ConfirmProvider';
import { useNotification } from '../../../../context/NotificationContext';

/**
 * Traduce un fallo de la API al motivo que el servidor dio.
 *
 * Antes todo terminaba en «Error al procesar la encuesta»: quien lo veía no podía
 * hacer nada con eso y quien lo reportaba tampoco. El servidor sí explica lo que pasó
 * —no hay destinatarios, no se pudo enviar a ninguno, la encuesta no existe—, así que
 * lo que hace falta es dejar de tragárselo.
 */
function motivoDelError(err: any, porDefecto: string): string {
  const data = err?.response?.data;
  const detalle = data?.error || data?.message;
  if (!detalle) {
    // Sin respuesta del servidor: o no llegó la petición, o cayó por el camino.
    return err?.message === 'Network Error'
      ? 'No se pudo conectar con el servidor. Revisa tu conexión e inténtalo otra vez.'
      : porDefecto;
  }
  // Los mensajes que llegan en inglés vienen de validaciones antiguas: se traducen
  // aquí para no dejar al usuario con un texto que no puede accionar.
  const traducciones: Record<string, string> = {
    'No recipients specified':
      'No elegiste destinatarios. Ábrela en Configuración y marca a quién va dirigida.',
    'Survey not found': 'Esa encuesta ya no existe: puede que la hayan borrado.',
    'Failed to parse recipient list':
      'La lista de destinatarios quedó corrupta. Vuelve a elegirlos en Configuración.',
  };
  return traducciones[detalle] || detalle;
}

interface SurveysProps {
  setHeaderAction: (node: React.ReactNode) => void;
}

const Surveys: React.FC<SurveysProps> = ({ setHeaderAction }) => {
  const [showBuilder, setShowBuilder] = useState(false);
  const [editingSurvey, setEditingSurvey] = useState<Survey | null>(null);
  const [viewingResultsFor, setViewingResultsFor] = useState<number | null>(null);
  const confirm = useConfirm();
  const notify = useNotification();
  const qc = useQueryClient();

  const { data: surveys = [], isLoading: loading } = useQuery<Survey[]>({
    queryKey: ['surveys'],
    queryFn: async () => (await surveyService.getSurveys()) || [],
  });
  const fetchSurveys = () => qc.invalidateQueries({ queryKey: ['surveys'] });

  useEffect(() => {
    if (!showBuilder && !editingSurvey && viewingResultsFor === null) {
      setHeaderAction(
        <button className={commonStyles['btn-primary']} onClick={() => setShowBuilder(true)}>
          <Plus size={16} /> Nueva Encuesta
        </button>
      );
    } else {
      setHeaderAction(null);
    }
    return () => setHeaderAction(null);
  }, [showBuilder, editingSurvey, viewingResultsFor]);

  const handleSaveSurvey = async (data: any, sendImmediately = false) => {
    try {
      const payload = {
        title: data.title,
        description: data.description,
        status: 'draft' as 'draft' | 'active' | 'closed',
        send_by_email: data.send_by_email,
        send_by_inapp: data.send_by_inapp,
        recipient_list: JSON.stringify(data.recipientIds || []),
        questions: data.questions,
      };

      let surveyId = data.id;

      if (surveyId) {
        await surveyService.updateSurvey(surveyId, { ...payload, id: surveyId });
      } else {
        const newSurvey = await surveyService.createSurvey(payload);
        surveyId = newSurvey.id;
      }

      if (sendImmediately && surveyId) {
        const res: any = await surveyService.sendSurvey(surveyId);
        // Un envío puede salir bien PARA UNOS y fallar para otros. Decir sólo
        // "enviada con éxito" esconde a quien se quedó fuera.
        const fallidos = res?.errors?.length ?? 0;
        if (fallidos > 0) {
          notify.warning(
            `Encuesta enviada a ${res?.sent ?? 0} persona(s), pero ${fallidos} no la recibieron. Revisa sus correos.`,
          );
        } else {
          notify.success(`Encuesta enviada a ${res?.sent ?? 0} persona(s).`);
        }
      } else {
        notify.success('Borrador guardado.');
      }

      setShowBuilder(false);
      setEditingSurvey(null);
      fetchSurveys();
    } catch (err) {
      console.error('Error saving/sending survey', err);
      notify.error(motivoDelError(err, 'No se pudo procesar la encuesta.'));
    }
  };

  const handleDeleteSurvey = async (surveyId: number) => {
    const ok = await confirm({
      title: 'Eliminar encuesta',
      message: '¿Estás seguro de que deseas eliminar esta encuesta?',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    });
    if (!ok) return;
    try {
      await surveyService.deleteSurvey(surveyId);
      fetchSurveys();
    } catch (err) {
      console.error('Error deleting survey', err);
      notify.error(motivoDelError(err, 'No se pudo eliminar la encuesta.'));
    }
  };

  // ---- Sub-view routing ----
  if (showBuilder || editingSurvey) {
    const builderData = editingSurvey
      ? { ...editingSurvey, questions: editingSurvey.questions || [] }
      : undefined;

    return (
      <SurveyBuilder
        initialData={builderData}
        onBack={() => { setShowBuilder(false); setEditingSurvey(null); }}
        onSave={(data) => handleSaveSurvey(data, false)}
        onSend={(data) => handleSaveSurvey(data, true)}
      />
    );
  }

  if (viewingResultsFor !== null) {
    return (
      <SurveyResults
        surveyId={viewingResultsFor}
        onBack={() => setViewingResultsFor(null)}
      />
    );
  }

  // ---- Helper ----
  const statusClass = (status: string) => {
    if (status === 'draft')  return listStyles.statusDraft;
    if (status === 'active') return listStyles.statusActive;
    return listStyles.statusClosed;
  };

  // ---- List View ----
  return (
    <div className={listStyles.surveysSection}>

      {loading ? (
        <p>Cargando encuestas...</p>
      ) : surveys.length === 0 ? (
        <div className={listStyles.emptyState}>
          <div className={listStyles.emptyIcon}>
            <ClipboardList size={32} />
          </div>
          <h3>Aún no tienes encuestas</h3>
          <p>Crea tu primera encuesta para recopilar feedback de tus Profesionales.</p>
        </div>
      ) : (
        <div className={listStyles.surveysList}>
          {surveys.map(survey => (
            <SurveyCard 
              key={survey.id}
              survey={survey}
              onEdit={setEditingSurvey}
              onViewResults={setViewingResultsFor}
              onDelete={handleDeleteSurvey}
              statusClass={statusClass}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export default Surveys;
