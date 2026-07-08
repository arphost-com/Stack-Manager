import { useEffect, useRef } from 'react';

// useFollowingScroll returns a ref to attach to a scrollable element
// (typically a <pre> that grows as a log streams in). While the user is
// pinned to the bottom of the viewport, the element auto-scrolls to the
// bottom whenever `contentKey` changes; once the user scrolls up to read
// history, auto-scroll pauses until they scroll back to the bottom.
// This is the standard "sticky tail" behavior — matches `tail -f`.
//
// contentKey: any value that changes when the log content updates (e.g.
// the log string itself, an array length, or a message-count counter).
// If a truthy `active` is passed and later flips to false, the ref will
// stop trying to auto-scroll — useful for pausing follow when a stream
// ends and the user should be free to browse.
export function useFollowingScroll(contentKey, active = true) {
  const ref = useRef(null);
  const followRef = useRef(true); // start pinned to bottom

  useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    const onScroll = () => {
      // 32px slack so a rounding-off-by-1 near the bottom still counts
      // as "user is following".
      const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 32;
      followRef.current = nearBottom;
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  useEffect(() => {
    if (!active) return;
    const el = ref.current;
    if (el && followRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [contentKey, active]);

  return ref;
}
