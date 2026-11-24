import { useLayoutEffect, useRef, useState, type ReactNode } from "react";

/**
 * Animates its own height to follow the content, so switching between contents of different
 * sizes (race card tabs, skeleton → data) grows or shrinks smoothly instead of jumping.
 */
export function AnimatedHeight({ children }: { children: ReactNode }) {
  const contentRef = useRef<HTMLDivElement>(null);
  const [contentHeight, setContentHeight] = useState<number | null>(null);

  useLayoutEffect(() => {
    const contentElement = contentRef.current;
    if (!contentElement) return;

    const measureContent = () => {
      // A hidden ancestor (display: none) measures 0: ignore it, otherwise coming back to the
      // view would animate from 0 to the real height.
      if (contentElement.getClientRects().length === 0) return;
      setContentHeight(contentElement.offsetHeight);
    };
    measureContent();
    const resizeObserver = new ResizeObserver(measureContent);
    resizeObserver.observe(contentElement);
    return () => resizeObserver.disconnect();
  }, []);

  // Until the first measurement the height is "auto": the first paint never animates.
  return (
    <div className="animated-height" style={{ height: contentHeight ?? "auto" }}>
      <div ref={contentRef} className="animated-height__content">
        {children}
      </div>
    </div>
  );
}
