import React, { useState, useEffect } from 'react';
import { ClipboardList, Plus } from 'lucide-react';
import listStyles from './SurveyList.module.css';
import commonStyles from '../Tools.module.css';
import SurveyBuilder from './SurveyBuilder';
import SurveyResults from './SurveyResults';
import SurveyCard from './components/SurveyCard';
import { surveyService, Survey } from '../../../../services/surveyService';

interface SurveysProps {
  setHeaderAction: (node: React.ReactNode) => void;
}

const Surveys: React.FC<SurveysProps> = ({ setHeaderAction }) => {
  const [surveys, setSurveys] = useState<Survey[]>([]);
  const [showBuilder, setShowBuilder] = useState(false);
  const [editingSurvey, setEditingSurvey] = useState<Survey | null>(null);
  const [viewingResultsFor, setViewingResultsFor] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchSurveys = async () => {
    try {
      const data = await surveyService.getSurveys();
      setSurveys(data || []);
    } catch (error) {
      console.error('Error fetching surveys:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSurveys();
  }, []);

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
        await surveyService.updateSurvey(surveyId, payload);
      } else {
        const newSurvey = await surveyService.createSurvey(payload);
        surveyId = newSurvey.id;
      }

      if (sendImmediately && surveyId) {
        await surveyService.sendSurvey(surveyId);
        alert('¡Encuesta enviada con éxito!');
      } else {
        alert('Borrador guardado.');
      }

      setShowBuilder(false);
      setEditingSurvey(null);
      fetchSurveys();
    } catch (err) {
      console.error('Error saving/sending survey', err);
      alert('Error al procesar la encuesta');
    }
  };

  const handleDeleteSurvey = async (surveyId: number) => {
    if (window.confirm('¿Estás seguro de que deseas eliminar esta encuesta?')) {
      try {
        await surveyService.deleteSurvey(surveyId);
        fetchSurveys();
      } catch (err) {
        console.error('Error deleting survey', err);
        alert('Error al eliminar la encuesta');
      }
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
          <p>Crea tu primera encuesta para recopilar feedback de tus empleados.</p>
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
