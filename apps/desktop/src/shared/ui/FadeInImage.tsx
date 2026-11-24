import { useState, type ImgHTMLAttributes } from "react";

type FadeInImageProps = ImgHTMLAttributes<HTMLImageElement> & {
  /** Called when the image cannot be loaded, so the caller can show a fallback. */
  onLoadFailed?: () => void;
};

/**
 * Fades in once decoded instead of painting top-to-bottom; a broken image (dead link, blocked
 * hotlink) disappears instead of showing the browser's broken-image icon.
 */
export function FadeInImage({ className = "", onLoadFailed, ...imageAttributes }: FadeInImageProps) {
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "failed">("loading");
  if (loadState === "failed") return null;
  return (
    <img
      {...imageAttributes}
      className={`fade-in-image ${loadState === "loaded" ? "fade-in-image--loaded" : ""} ${className}`}
      onLoad={() => setLoadState("loaded")}
      onError={() => {
        setLoadState("failed");
        onLoadFailed?.();
      }}
    />
  );
}
