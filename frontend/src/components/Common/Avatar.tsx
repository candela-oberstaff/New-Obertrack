import React, { useState, useEffect } from 'react';
import styles from './Avatar.module.css';

interface AvatarProps {
  src?: string;
  name?: string;
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
  className?: string;
  borderColor?: string;
}

const Avatar: React.FC<AvatarProps> = ({ 
  src, 
  name = '?', 
  size = 'md', 
  className = '',
  borderColor
}) => {
  const [error, setError] = useState(false);

  // Reset error state when src changes
  useEffect(() => {
    setError(false);
  }, [src]);

  const initials = name
    .split(' ')
    .map(n => n[0])
    .join('')
    .slice(0, 2)
    .toUpperCase();

  const containerClasses = [
    styles.avatar,
    styles[size],
    className
  ].join(' ');

  const containerStyle: React.CSSProperties = borderColor 
    ? { border: `2px solid ${borderColor}` } 
    : {};

  return (
    <div className={containerClasses} style={containerStyle} title={name}>
      {src && !error ? (
        <img
          src={src}
          alt={name}
          className={styles.image}
          onError={() => setError(true)}
        />
      ) : (
        <div className={styles.placeholder}>
          {initials}
        </div>
      )}
    </div>
  );
};

export default Avatar;
