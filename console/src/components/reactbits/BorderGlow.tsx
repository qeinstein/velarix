import { useRef, useEffect, ReactNode } from 'react';

interface BorderGlowProps {
  className?: string;
  glowColor?: string;
  glowSize?: number;
  animationSpeed?: number;
}

const BorderGlow = ({
  children,
  config = {},
}: {
  children: ReactNode;
  config?: BorderGlowProps;
}) => {
  const {
    className = '',
    glowColor = 'rgba(6, 182, 212, 0.6)',
    glowSize = 2,
    animationSpeed = 3,
  } = config;

  const containerRef = useRef<HTMLDivElement>(null);
  const glowRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    const glow = glowRef.current;
    if (!container || !glow) return;

    const handleMouseMove = (e: MouseEvent) => {
      const rect = container.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      glow.style.setProperty('--glow-x', `${x}px`);
      glow.style.setProperty('--glow-y', `${y}px`);
      glow.style.opacity = '1';
    };

    const handleMouseLeave = () => {
      glow.style.opacity = '0';
    };

    container.addEventListener('mousemove', handleMouseMove);
    container.addEventListener('mouseleave', handleMouseLeave);

    return () => {
      container.removeEventListener('mousemove', handleMouseMove);
      container.removeEventListener('mouseleave', handleMouseLeave);
    };
  }, []);

  return (
    <div
      ref={containerRef}
      className={`border-glow-wrapper ${className}`}
      style={{
        position: 'relative',
        overflow: 'hidden',
        borderRadius: 'inherit',
      }}
    >
      {/* Animated border glow that orbits */}
      <div
        className="border-glow-orbit"
        style={{
          position: 'absolute',
          inset: -glowSize,
          borderRadius: 'inherit',
          padding: glowSize,
          background: `conic-gradient(from var(--glow-angle, 0deg), transparent 0%, ${glowColor} 10%, transparent 20%, transparent 50%, ${glowColor} 60%, transparent 70%)`,
          WebkitMask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
          WebkitMaskComposite: 'xor',
          maskComposite: 'exclude',
          animation: `borderGlowSpin ${animationSpeed}s linear infinite`,
          pointerEvents: 'none',
          zIndex: 1,
        }}
      />

      {/* Mouse-follow glow */}
      <div
        ref={glowRef}
        style={{
          position: 'absolute',
          inset: -glowSize,
          borderRadius: 'inherit',
          padding: glowSize,
          background: `radial-gradient(300px circle at var(--glow-x, 50%) var(--glow-y, 50%), ${glowColor}, transparent 70%)`,
          WebkitMask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
          WebkitMaskComposite: 'xor',
          maskComposite: 'exclude',
          opacity: 0,
          transition: 'opacity 0.3s ease',
          pointerEvents: 'none',
          zIndex: 2,
        }}
      />

      {/* Content */}
      <div style={{ position: 'relative', zIndex: 3, borderRadius: 'inherit' }}>
        {children}
      </div>
    </div>
  );
};

export default BorderGlow;
