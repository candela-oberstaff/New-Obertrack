import React from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

const containerStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: 6,
  padding: '16px 0',
};

const btnStyle: React.CSSProperties = {
  width: 32,
  height: 32,
  borderRadius: 8,
  border: '1px solid #e2e8f0',
  background: '#fff',
  color: '#475569',
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: 13,
  fontWeight: 600,
  transition: 'all 0.15s',
};

const activeBtnStyle: React.CSSProperties = {
  ...btnStyle,
  background: '#7c3aed',
  color: '#fff',
  borderColor: '#7c3aed',
};

const arrowBtnStyle: React.CSSProperties = {
  ...btnStyle,
  width: 36,
};

const Pagination: React.FC<PaginationProps> = ({ currentPage, totalPages, onPageChange }) => {
  if (totalPages <= 1) return null;

  const getVisiblePages = (): (number | 'ellipsis')[] => {
    const pages: (number | 'ellipsis')[] = [];
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) pages.push(i);
    } else {
      pages.push(1);
      if (currentPage > 3) pages.push('ellipsis');
      const start = Math.max(2, currentPage - 1);
      const end = Math.min(totalPages - 1, currentPage + 1);
      for (let i = start; i <= end; i++) pages.push(i);
      if (currentPage < totalPages - 2) pages.push('ellipsis');
      pages.push(totalPages);
    }
    return pages;
  };

  return (
    <div style={containerStyle}>
      <button
        style={{ ...arrowBtnStyle, opacity: currentPage === 1 ? 0.4 : 1 }}
        disabled={currentPage === 1}
        onClick={() => onPageChange(currentPage - 1)}
      >
        <ChevronLeft size={15} />
      </button>

      {getVisiblePages().map((page, idx) =>
        page === 'ellipsis' ? (
          <span key={`e${idx}`} style={{ color: '#94a3b8', fontSize: 13, padding: '0 4px' }}>...</span>
        ) : (
          <button
            key={page}
            style={page === currentPage ? activeBtnStyle : btnStyle}
            onClick={() => onPageChange(page)}
            onMouseEnter={e => {
              if (page !== currentPage) {
                (e.currentTarget as HTMLElement).style.background = '#f1f5f9';
              }
            }}
            onMouseLeave={e => {
              if (page !== currentPage) {
                (e.currentTarget as HTMLElement).style.background = '#fff';
              }
            }}
          >
            {page}
          </button>
        )
      )}

      <button
        style={{ ...arrowBtnStyle, opacity: currentPage === totalPages ? 0.4 : 1 }}
        disabled={currentPage === totalPages}
        onClick={() => onPageChange(currentPage + 1)}
      >
        <ChevronRight size={15} />
      </button>
    </div>
  );
};

export default Pagination;
