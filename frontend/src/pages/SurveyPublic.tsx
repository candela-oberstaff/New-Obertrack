import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { AlertTriangle, CheckCircle2 } from 'lucide-react';

import SurveyQuestions, { type SurveyAnswers } from '../components/Surveys/SurveyQuestions';
import { publicSurveyService, buildAnswerPayload, type PublicSurvey } from '../services/surveyService';
import styles from './SurveyPublic.module.css';

/**
 * Encuesta respondida SIN iniciar sesión, desde el enlace que la empresa recibió
 * por correo.
 *
 * Existe solo para las cuentas empresa: el backend valida el rol contra el token
 * y esta página no decide nada al respecto. Para los demás roles la encuesta se
 * sigue respondiendo dentro de la aplicación (SurveyViewer).
 *
 * Vive fuera del Layout autenticado —no hay menú, ni sesión, ni a dónde
 * "volver"—, así que trae su propia envoltura de marca, igual que la inducción y
 * el testimonio.
 */

function errorMessage(err: unknown, fallback: string): string {
  const responseError =
    err && typeof err === 'object' && 'response' in err
      ? (err as { response?: { data?: { error?: string } } }).response?.data?.error
      : undefined;
  return responseError || fallback;
}

const SurveyPublic: React.FC = () => {
  const { token = '' } = useParams<{ token: string }>();

  const [survey, setSurvey] = useState<PublicSurvey | null>(null);
  const [answers, setAnswers] = useState<SurveyAnswers>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [submitError, setSubmitError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const data = await publicSurveyService.get(token);
        if (!cancelled) setSurvey(data);
      } catch (err) {
        if (!cancelled) {
          setLoadError(errorMessage(err, 'No pudimos abrir la encuesta con este enlace.'));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => {
      cancelled = true;
    };
  }, [token]);

  const handleAnswerChange = (questionId: number, value: any) => {
    setAnswers(prev => ({ ...prev, [questionId]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!survey || submitting) return;

    setSubmitting(true);
    setSubmitError('');
    try {
      await publicSurveyService.submit(token, buildAnswerPayload(survey.questions || [], answers));
      setSubmitted(true);
    } catch (err) {
      setSubmitError(errorMessage(err, 'No se pudieron guardar tus respuestas. Vuelve a intentarlo.'));
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.state}>
          <p className={styles.stateText}>Cargando la encuesta...</p>
        </div>
      </div>
    );
  }

  // El enlace venció, ya no vale o la encuesta se cerró. El motivo lo escribe el
  // servidor: cada uno tiene su salida y no sirve el mismo texto para todos.
  if (loadError || !survey) {
    return (
      <div className={styles.page}>
        <div className={styles.state}>
          <AlertTriangle size={48} className={styles.stateIcon} />
          <h1 className={styles.stateTitle}>No pudimos abrir la encuesta</h1>
          <p className={styles.stateText}>{loadError || 'Este enlace no es válido.'}</p>
        </div>
      </div>
    );
  }

  if (submitted) {
    return (
      <div className={styles.page}>
        <div className={styles.state}>
          <CheckCircle2 size={48} className={styles.stateIcon} />
          <h1 className={styles.stateTitle}>¡Gracias por responder!</h1>
          <p className={styles.stateText}>
            Registramos tus respuestas. Ya puedes cerrar esta página.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.page}>
      <div className={styles.shell}>
        <header className={styles.header}>
          <img src="/logos/Horizontal_Blanco.png" alt="Obertrack" className={styles.logo} />
          <p className={styles.kicker}>Encuesta</p>
          <h1 className={styles.title}>{survey.title}</h1>
          {survey.description && <p className={styles.description}>{survey.description}</p>}
        </header>

        {survey.recipient_name && (
          <p className={styles.greeting}>
            Hola <strong>{survey.recipient_name}</strong>, gracias por tomarte unos minutos.
          </p>
        )}

        {survey.already_answered && (
          <p className={styles.answeredNotice}>
            Ya recibimos una respuesta tuya a esta encuesta. Si la completas aquí, lo que envíes
            ahora reemplazará a lo anterior.
          </p>
        )}

        {submitError && <p className={styles.error}>{submitError}</p>}

        <SurveyQuestions
          questions={survey.questions || []}
          answers={answers}
          onChange={handleAnswerChange}
          onSubmit={handleSubmit}
          submitLabel="Enviar respuestas"
          submitting={submitting}
        />

        <p className={styles.foot}>Este enlace es personal. No lo compartas.</p>
      </div>
    </div>
  );
};

export default SurveyPublic;
