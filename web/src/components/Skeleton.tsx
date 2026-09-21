/**
 * Placeholders for the first load.
 *
 * An empty screen reads as "there is nothing here" — in place of machines or
 * containers that is a plain lie while the NAS answer is still in flight. Grey
 * bars shaped like the future contents are honest: the space is taken, and by what.
 *
 * The placeholders are built from the same classes as the real cards, so the
 * contents take their place without the layout jumping.
 *
 * They show only when there is nothing to show: if the data is already there
 * (from the cache or an earlier poll), the refresh happens quietly.
 */

import { t } from '../i18n'

/** Row widths in a cycle: identical bars look like a table, not like text. */
const WIDTHS = ['64%', '46%', '73%', '54%', '68%']

interface BarProps {
  width: string
  height?: number
}

/** A placeholder bar in place of a line of text. */
export function Bar({ width, height = 13 }: BarProps) {
  return <span className="sk" style={{ width, height }} />
}

interface RowsProps {
  /** How many cards to draw. */
  count?: number
  /** A dot on the left — a state indicator, as on machines and containers. */
  dot?: boolean
  /** A round button on the right. */
  action?: boolean
}

/** A list of cards: a name, a caption and, if needed, a button. */
export function SkeletonRows({ count = 3, dot = false, action = false }: RowsProps) {
  return (
    <div className="list" role="status" aria-label={t('common.loading')}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="card vm" aria-hidden="true">
          <div className="vm-head">
            {dot && <span className="sk sk-dot" />}
            <div className="vm-text">
              <Bar width={WIDTHS[i % WIDTHS.length]} height={15} />
              <Bar width="34%" height={11} />
            </div>
            {action && <span className="sk sk-button" />}
          </div>
        </div>
      ))}
    </div>
  )
}

/** Two summary tiles — "Free RAM", "vCPU in use" and the like. */
export function SkeletonTiles({ count = 2 }: { count?: number }) {
  return (
    <div className="tiles" aria-hidden="true">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="tile">
          <Bar width="72%" height={11} />
          <Bar width="48%" height={17} />
        </div>
      ))}
    </div>
  )
}

/** Log rows: a dot, a two-line message and a time. */
export function SkeletonEvents({ count = 5 }: { count?: number }) {
  return (
    <div className="list" role="status" aria-label={t('common.loading')}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="card event" aria-hidden="true">
          <span className="sk sk-dot" />
          <div className="event-text">
            <Bar width={WIDTHS[i % WIDTHS.length]} height={13} />
            <Bar width="28%" height={11} />
          </div>
        </div>
      ))}
    </div>
  )
}

/** The CPU and RAM meters on the home screen. */
export function SkeletonMeters() {
  return (
    <div aria-hidden="true">
      {['CPU', 'RAM'].map((label) => (
        <div key={label} className="meter">
          <span className="meter-label">{label}</span>
          <span className="sk meter-track" />
          <span className="meter-value"><Bar width="100%" height={11} /></span>
        </div>
      ))}
    </div>
  )
}

/** File list rows: an icon, a name and a size. */
export function SkeletonEntries({ count = 6 }: { count?: number }) {
  return (
    <div className="list" role="status" aria-label={t('common.loading')}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="card entry" aria-hidden="true">
          <div className="entry-main">
            <span className="sk sk-icon" />
            <span className="entry-text sk-grow">
              <Bar width={WIDTHS[i % WIDTHS.length]} height={14} />
              <Bar width="22%" height={11} />
            </span>
          </div>
        </div>
      ))}
    </div>
  )
}

/** Download cards: a progress ring, a name and a status line. */
export function SkeletonTasks({ count = 3 }: { count?: number }) {
  return (
    <div className="list" role="status" aria-label={t('common.loading')}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="card task" aria-hidden="true">
          <div className="task-head">
            <span className="sk sk-ring" />
            <div className="entry-text sk-grow">
              <Bar width={WIDTHS[i % WIDTHS.length]} height={14} />
              <Bar width="42%" height={11} />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

/** Disk cards in the "Disks" list. */
export function SkeletonDisks({ count = 4 }: { count?: number }) {
  return (
    <div className="card" role="status" aria-label={t('common.loading')}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} aria-hidden="true">
          {i > 0 && <div className="divider" />}
          <div className="disk">
            <span className="sk sk-dot" />
            <div className="disk-text">
              <Bar width={WIDTHS[i % WIDTHS.length]} height={13} />
              <Bar width="30%" height={11} />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
