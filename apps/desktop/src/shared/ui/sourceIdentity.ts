/**
 * Visual identity for a news source without fetching anything external (a favicon service would
 * tell a third party which sites the user reads): a stable hue derived from the source slug,
 * plus its initial.
 */
export function sourceHue(sourceSlug: string): number {
  let hashValue = 0;
  for (const character of sourceSlug) {
    hashValue = (hashValue * 31 + character.charCodeAt(0)) >>> 0;
  }
  return hashValue % 360;
}

export function sourceInitial(sourceName: string): string {
  const firstLetter = sourceName.trim().match(/\p{L}|\p{N}/u)?.[0];
  return (firstLetter ?? "?").toLocaleUpperCase();
}

/** "https://www.motorsport.com/f1/news/x" → "motorsport.com": shown under the main button. */
export function displayDomain(articleUrl: string): string {
  try {
    return new URL(articleUrl).hostname.replace(/^www\./, "");
  } catch {
    return "";
  }
}
