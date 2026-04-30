import { useState, useEffect, useCallback, type RefObject } from 'react';

export function useScrollToBottom(containerRef: RefObject<HTMLElement | null>) {
  const [isAtBottom, setIsAtBottom] = useState(true);

  const checkScroll = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    const threshold = 20;
    const atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - threshold;
    setIsAtBottom(atBottom);
  }, [containerRef]);

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    const el = containerRef.current;
    if (!el) return;
    el.scrollTo({ top: el.scrollHeight, behavior });
    setIsAtBottom(true);
  }, [containerRef]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    el.addEventListener('scroll', checkScroll);
    checkScroll();
    return () => el.removeEventListener('scroll', checkScroll);
  }, [containerRef, checkScroll]);

  return { isAtBottom, scrollToBottom, checkScroll };
}
