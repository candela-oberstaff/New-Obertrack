import React, { lazy, Suspense } from 'react';

const MetricsPage = lazy(() => import('../../../pages/Metrics'));

const Metrics: React.FC = () => {
  return (
    <Suspense fallback={<div>Cargando Métricas...</div>}>
      <MetricsPage />
    </Suspense>
  );
};

export default Metrics;
