interface Props {
  /** Доля от 0 до 1. */
  value: number
  color: string
  size?: number
  /** Символ в центре кольца. */
  glyph?: string
}

/**
 * Кольцевой индикатор: занимает меньше места, чем полоска с подписью, и
 * читается с одного взгляда даже в плотном списке.
 */
export function ProgressRing({ value, color, size = 44, glyph }: Props) {
  const stroke = size >= 100 ? 10 : 4
  const radius = size / 2 - stroke / 2 - 1
  const circumference = 2 * Math.PI * radius
  const filled = circumference * Math.min(Math.max(value, 0), 1)

  return (
    <div style={{ position: 'relative', width: size, height: size, flexShrink: 0 }}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden="true">
        <circle
          cx={size / 2} cy={size / 2} r={radius}
          fill="none" stroke="var(--ring-track)" strokeWidth={stroke}
        />
        <circle
          cx={size / 2} cy={size / 2} r={radius}
          fill="none" stroke={color} strokeWidth={stroke} strokeLinecap="round"
          strokeDasharray={`${filled} ${circumference - filled}`}
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
        />
      </svg>
      {glyph && (
        <div
          style={{
            position: 'absolute', inset: 0, display: 'flex',
            alignItems: 'center', justifyContent: 'center',
            fontSize: size >= 100 ? 30 : 13, fontWeight: 700, color,
          }}
        >
          {glyph}
        </div>
      )}
    </div>
  )
}
