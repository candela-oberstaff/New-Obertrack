import React, { useRef, useState, useEffect, useCallback, type ReactNode } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import './ScrollableTabs.css'

export interface TabItem<T extends string = string> {
  id: T
  label: string
  count?: number | string
  icon?: ReactNode
  badgeColor?: string
  disabled?: boolean
  title?: string
}

export interface ScrollableTabsProps<T extends string = string> {
  tabs: TabItem<T>[]
  activeTab: T
  onChange: (tabId: T) => void
  size?: 'sm' | 'md'
  variant?: 'primary' | 'secondary' | 'pills'
  className?: string
  containerClassName?: string
  ariaLabel?: string
  showArrows?: boolean
}

export function ScrollableTabs<T extends string = string>({
  tabs,
  activeTab,
  onChange,
  size = 'md',
  variant = 'primary',
  className = '',
  containerClassName = '',
  ariaLabel = 'Pestañas de navegación',
  showArrows = true,
}: ScrollableTabsProps<T>) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)
  const isDraggingRef = useRef(false)
  const dragStartXRef = useRef(0)
  const scrollStartRef = useRef(0)
  const hasDraggedRef = useRef(false)

  const updateScrollState = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    const { scrollLeft, scrollWidth, clientWidth } = el
    const maxScroll = scrollWidth - clientWidth
    setCanScrollLeft(scrollLeft > 2)
    setCanScrollRight(maxScroll - scrollLeft > 2)
  }, [])

  // Listen to resize, content changes and scroll events
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return

    updateScrollState()

    const handleScroll = () => updateScrollState()
    el.addEventListener('scroll', handleScroll, { passive: true })

    const ro = new ResizeObserver(() => {
      updateScrollState()
    })
    ro.observe(el)

    const handleWindowResize = () => updateScrollState()
    window.addEventListener('resize', handleWindowResize)

    return () => {
      el.removeEventListener('scroll', handleScroll)
      ro.disconnect()
      window.removeEventListener('resize', handleWindowResize)
    }
  }, [updateScrollState, tabs])

  // Center or reveal active tab on tab change
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return

    const activeEl = el.querySelector<HTMLElement>('[data-active="true"]')
    if (activeEl) {
      const containerRect = el.getBoundingClientRect()
      const activeRect = activeEl.getBoundingClientRect()

      // Scroll if active tab is partly or fully clipped
      if (activeRect.left < containerRect.left + 40 || activeRect.right > containerRect.right - 40) {
        const targetScroll =
          el.scrollLeft +
          (activeRect.left - containerRect.left) -
          (containerRect.width / 2) +
          (activeRect.width / 2)
        el.scrollTo({ left: Math.max(0, targetScroll), behavior: 'smooth' })
      }
    }
  }, [activeTab])

  const scrollByAmount = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const amount = Math.max(el.clientWidth * 0.65, 200)
    el.scrollBy({
      left: direction === 'left' ? -amount : amount,
      behavior: 'smooth',
    })
  }

  // Handle mouse wheel horizontal scroll
  const handleWheel = (e: React.WheelEvent<HTMLDivElement>) => {
    const el = scrollRef.current
    if (!el) return
    if (el.scrollWidth > el.clientWidth && Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
      el.scrollLeft += e.deltaY
    }
  }

  // Mouse drag handlers
  const handleMouseDown = (e: React.MouseEvent<HTMLDivElement>) => {
    const el = scrollRef.current
    if (!el) return
    // Only left click
    if (e.button !== 0) return
    isDraggingRef.current = true
    hasDraggedRef.current = false
    dragStartXRef.current = e.pageX - el.offsetLeft
    scrollStartRef.current = el.scrollLeft
  }

  const handleMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!isDraggingRef.current) return
    const el = scrollRef.current
    if (!el) return
    e.preventDefault()
    const x = e.pageX - el.offsetLeft
    const walk = (x - dragStartXRef.current) * 1.2
    if (Math.abs(walk) > 4) {
      hasDraggedRef.current = true
    }
    el.scrollLeft = scrollStartRef.current - walk
  }

  const handleMouseUp = () => {
    isDraggingRef.current = false
  }

  const handleMouseLeave = () => {
    isDraggingRef.current = false
  }

  return (
    <div
      className={`ui-scrollable-tabs-container ${containerClassName}`}
      role="region"
      aria-label={ariaLabel}
    >
      {showArrows && (
        <button
          type="button"
          className={`ui-scrollable-tabs__arrow ui-scrollable-tabs__arrow--left ${
            canScrollLeft ? 'ui-scrollable-tabs__arrow--visible' : ''
          }`}
          onClick={() => scrollByAmount('left')}
          disabled={!canScrollLeft}
          aria-label="Desplazar opciones hacia la izquierda"
          tabIndex={canScrollLeft ? 0 : -1}
          title="Ver opciones anteriores"
        >
          <ChevronLeft size={18} />
        </button>
      )}

      {showArrows && canScrollLeft && (
        <div className="ui-scrollable-tabs__fade ui-scrollable-tabs__fade--left" />
      )}

      <div
        ref={scrollRef}
        className={`ui-scrollable-tabs ${size === 'sm' ? 'ui-scrollable-tabs--sm' : ''} ${
          variant === 'pills'
            ? 'ui-scrollable-tabs--pills'
            : variant === 'secondary'
            ? 'ui-scrollable-tabs--secondary'
            : 'ui-scrollable-tabs--primary'
        } ${className}`}
        role="tablist"
        onWheel={handleWheel}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseLeave}
      >
        {tabs.map(tab => {
          const isActive = activeTab === tab.id
          return (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={isActive}
              data-active={isActive}
              disabled={tab.disabled}
              title={tab.title || tab.label}
              className={`ui-scrollable-tab ${isActive ? 'ui-scrollable-tab--active' : ''} ${
                tab.disabled ? 'ui-scrollable-tab--disabled' : ''
              }`}
              onClick={e => {
                if (hasDraggedRef.current) {
                  e.preventDefault()
                  return
                }
                if (!tab.disabled) {
                  onChange(tab.id)
                }
              }}
            >
              {tab.icon && <span className="ui-scrollable-tab__icon">{tab.icon}</span>}
              <span className="ui-scrollable-tab__label">{tab.label}</span>
              {tab.count !== undefined && tab.count !== null && (
                <span
                  className="ui-scrollable-tab__count"
                  style={tab.badgeColor ? { backgroundColor: tab.badgeColor } : undefined}
                >
                  {tab.count}
                </span>
              )}
            </button>
          )
        })}
      </div>

      {showArrows && canScrollRight && (
        <div className="ui-scrollable-tabs__fade ui-scrollable-tabs__fade--right" />
      )}

      {showArrows && (
        <button
          type="button"
          className={`ui-scrollable-tabs__arrow ui-scrollable-tabs__arrow--right ${
            canScrollRight ? 'ui-scrollable-tabs__arrow--visible' : ''
          }`}
          onClick={() => scrollByAmount('right')}
          disabled={!canScrollRight}
          aria-label="Desplazar opciones hacia la derecha"
          tabIndex={canScrollRight ? 0 : -1}
          title="Ver más opciones"
        >
          <ChevronRight size={18} />
        </button>
      )}
    </div>
  )
}
