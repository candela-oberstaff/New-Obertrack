import React from 'react';
import { Edit2, Users, Trash2 } from 'lucide-react';
import listStyles from '../SurveyList.module.css';
import commonStyles from '../../Tools.module.css';
import { Survey } from '../../../../../services/surveyService';

interface SurveyCardProps {
  survey: Survey;
  onEdit: (survey: Survey) => void;
  onViewResults: (id: number) => void;
  onDelete: (id: number) => void;
  statusClass: (status: string) => string;
}

const SurveyCard: React.FC<SurveyCardProps> = ({ survey, onEdit, onViewResults, onDelete, statusClass }) => {
  return (
    <div className={listStyles.surveyCard}>
      <div className={listStyles.surveyInfo}>
        <h3>{survey.title}</h3>
        <p className={listStyles.surveyMeta}>
          Estado: <span className={statusClass(survey.status)}>{survey.status}</span>
        </p>
        <p className={listStyles.surveyMeta}>
          <Users size={14} /> Respuestas: {survey.responses ? survey.responses.length : 0}
        </p>
      </div>
      <div className={listStyles.surveyActions}>
        <button className={commonStyles['btn-outline']} onClick={() => onEdit(survey)}>
          <Edit2 size={16} /> Editar
        </button>
        <button className={commonStyles['btn-outline']} onClick={() => onViewResults(survey.id!)}>
          Ver Resultados
        </button>
        <button 
          className={commonStyles['btn-outline']} 
          style={{ color: '#ef4444', borderColor: '#fca5a5', background: '#fef2f2' }}
          onClick={() => onDelete(survey.id!)}
          title="Eliminar encuesta"
        >
          <Trash2 size={16} />
        </button>
      </div>
    </div>
  );
};

export default SurveyCard;
