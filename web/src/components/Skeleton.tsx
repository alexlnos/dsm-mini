/**
 * Заглушки на время первой загрузки.
 *
 * Пустой экран читается как «здесь ничего нет» — на месте машин или
 * контейнеров это прямая ложь, пока ответ NAS ещё в пути. Серые полоски
 * в форме будущего содержимого честнее: видно, что место занято, и чем.
 *
 * Заглушки собраны из тех же классов, что и настоящие карточки, поэтому
 * содержимое встаёт на их место без скачка вёрстки.
 *
 * Показываются только когда показывать нечего: если данные уже есть (из
 * кэша или прошлого опроса), обновление идёт молча, без мигания.
 */

import { t } from '../i18n'

/** Ширины строк по кругу: одинаковые полоски выглядят как таблица, а не текст. */
const WIDTHS = ['64%', '46%', '73%', '54%', '68%']

interface BarProps {
  width: string
  height?: number
}

/** Полоска-заглушка на месте строки текста. */
export function Bar({ width, height = 13 }: BarProps) {
  return <span className="sk" style={{ width, height }} />
}

interface RowsProps {
  /** Сколько карточек нарисовать. */
  count?: number
  /** Кружок слева — индикатор состояния, как у машин и контейнеров. */
  dot?: boolean
  /** Круглая кнопка справа. */
  action?: boolean
}

/** Список карточек: имя, подпись и, если нужно, кнопка. */
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

/** Две плитки сводки — «Свободно RAM», «Занято vCPU» и подобные. */
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

/** Строки журнала: точка, сообщение в две строки и время. */
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

/** Шкалы CPU и RAM на главном экране. */
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

/** Строки файлового списка: значок, имя и размер. */
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

/** Карточки загрузок: кольцо прогресса, имя и строка состояния. */
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

/** Карточки дисков в списке «Диски». */
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
