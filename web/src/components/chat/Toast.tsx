type Props = { message: string; onDismiss: () => void };

export default function Toast({ message, onDismiss }: Props) {
  return (
    <div
      role="alert"
      style={{
        position: 'fixed',
        bottom: '1rem',
        right: '1rem',
        background: 'var(--bg-topbar, #161b22)',
        border: '1px solid var(--border, #30363d)',
        color: 'var(--fg, #c9d1d9)',
        padding: '0.5rem 1rem',
        borderRadius: 4,
        boxShadow: '0 4px 16px rgba(0,0,0,0.3)',
        zIndex: 1000,
      }}
    >
      {message}
      <button
        type="button"
        onClick={onDismiss}
        style={{
          marginLeft: '1rem',
          background: 'transparent',
          border: 'none',
          color: 'inherit',
          cursor: 'pointer',
        }}
        aria-label="dismiss"
      >
        ×
      </button>
    </div>
  );
}
