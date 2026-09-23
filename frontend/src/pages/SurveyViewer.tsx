import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { surveyService, buildAnswerPayload, Survey } from '../services/surveyService';
import SurveyQuestions, { type SurveyAnswers } from '../components/Surveys/SurveyQuestions';
import styles from './SurveyViewer.module.css';
import { CheckCircle2 } from 'lucide-react';

/**
 * Encuesta respondida DENTRO de la aplicación, con sesión iniciada. Es la vía de
 * los profesionales y del equipo interno; las empresas tienen además la página
 * pública (SurveyPublic), que se abre desde el correo sin iniciar sesión.
 */
const SurveyViewer: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [survey, setSurvey] = useState<Survey | null>(null);
  const [answers, setAnswers] = useState<SurveyAnswers>({});
  const [loading, setLoading] = useState(true);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchSurvey = async () => {
      try {
        if (id) {
          const data = await surveyService.getSurvey(parseInt(id));
          setSurvey(data);
        }
      } catch (err) {
        setError('No se pudo cargar la encuesta. Es posible que no exista o no tengas acceso.');
      } finally {
        setLoading(false);
      }
    };
    fetchSurvey();
  }, [id]);

  const handleAnswerChange = (questionId: number, value: any) => {
    setAnswers(prev => ({
      ...prev,
      [questionId]: value
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!survey || !survey.id) return;

    try {
      await surveyService.submitResponse(survey.id, buildAnswerPayload(survey.questions || [], answers));
      setSubmitted(true);
    } catch (err) {
      alert("Hubo un error al enviar tus respuestas. Por favor, intenta de nuevo.");
    }
  };

  if (loading) {
    return <div className={styles.loadingState}>Cargando encuesta...</div>;
  }

  if (error || !survey) {
    return (
      <div className={styles.errorState}>
        <h2>Error</h2>
        <p>{error || 'Encuesta no encontrada'}</p>
        <button onClick={() => navigate('/')}>Volver al inicio</button>
      </div>
    );
  }

  if (submitted) {
    return (
      <div className={styles.successState}>
        <CheckCircle2 size={64} className={styles.successIcon} />
        <h2>¡Gracias por tus respuestas!</h2>
        <p>Tu participación es muy importante para nosotros.</p>
        <button onClick={() => navigate('/dashboard')}>Ir al Panel Principal</button>
      </div>
    );
  }

  return (
    <div className={styles.viewerContainer}>
      <div className={styles.surveyPaper}>
        <header className={styles.surveyHeader}>
          <h1>{survey.title}</h1>
          {survey.description && <p className={styles.surveyDescription}>{survey.description}</p>}
        </header>

        <SurveyQuestions
          questions={survey.questions || []}
          answers={answers}
          onChange={handleAnswerChange}
          onSubmit={handleSubmit}
        />
      </div>
    </div>
  );
};

export default SurveyViewer;
