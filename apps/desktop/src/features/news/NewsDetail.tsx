import { useEffect, useState, type CSSProperties } from "react";
import type { Article } from "../../shared/api/types";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { openExternalUrl } from "../../shared/platform/tauriBridge";
import { formatRelativeTime } from "../../shared/time/timeZone";
import { FadeInImage } from "../../shared/ui/FadeInImage";
import { BackIcon, ChevronRightIcon, ExternalLinkIcon, LinkIcon, UnreadDotIcon } from "../../shared/ui/icons";
import { displayDomain, sourceHue, sourceInitial } from "../../shared/ui/sourceIdentity";
import { useSettings } from "../settings/SettingsProvider";

const COPIED_FEEDBACK_MS = 1800;

export type ArticleNavigationDirection = "forward" | "back";

type NewsDetailProps = {
  article: Article;
  previousArticle?: Article;
  nextArticle?: Article;
  onNavigate: (targetArticle: Article, direction: ArticleNavigationDirection) => void;
  onMarkRead: (articleId: string) => Promise<void>;
  onMarkUnread: (articleId: string) => Promise<void>;
};

/** Shows what the feed publishes (headline, summary, image); the full article opens on the source site. */
export function NewsDetail({ article, previousArticle, nextArticle, onNavigate, onMarkRead, onMarkUnread }: NewsDetailProps) {
  const { messages, locale } = useLocale();
  const { effectiveTimeZone } = useSettings();
  // Opening an article marks it as read, so the detail always starts in the "read" state.
  const [isUnread, setIsUnread] = useState(false);
  const [isLinkCopied, setIsLinkCopied] = useState(false);

  useEffect(() => {
    if (!isLinkCopied) return;
    const feedbackTimer = window.setTimeout(() => setIsLinkCopied(false), COPIED_FEEDBACK_MS);
    return () => window.clearTimeout(feedbackTimer);
  }, [isLinkCopied]);

  const publishedAtText = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short", timeZone: effectiveTimeZone }).format(
    new Date(article.publishedAt),
  );
  const sourceStyle = { "--source-hue": sourceHue(article.sourceSlug) } as CSSProperties;
  const articleDomain = displayDomain(article.articleUrl);

  async function copyArticleLink() {
    try {
      await navigator.clipboard.writeText(article.articleUrl);
      setIsLinkCopied(true);
    } catch {
      setIsLinkCopied(false);
    }
  }

  function toggleReadState() {
    const nextIsUnread = !isUnread;
    setIsUnread(nextIsUnread);
    (nextIsUnread ? onMarkUnread(article.id) : onMarkRead(article.id)).catch(() => setIsUnread(!nextIsUnread));
  }

  return (
    <article className="news-detail" style={sourceStyle}>
      <NewsHero article={article} />

      <header className="news-detail__byline">
        <span className="source-avatar" aria-hidden="true">
          {sourceInitial(article.sourceName)}
        </span>
        <span className="news-detail__byline-text">
          <span className="news-detail__source">
            {article.sourceName}
            {article.languageCode !== locale.slice(0, 2) && <span className="news-item__language">{article.languageCode.toUpperCase()}</span>}
          </span>
          <time className="news-detail__date" dateTime={article.publishedAt} title={messages.publishedAtLabel(publishedAtText)}>
            {publishedAtText} · {formatRelativeTime(article.publishedAt, locale)}
          </time>
        </span>
      </header>

      <h2 className="news-detail__title">{article.title}</h2>

      {article.summary && (
        <>
          <p className="news-detail__summary">{article.summary}</p>
          <p className="news-detail__attribution">{messages.summaryAttribution(article.sourceName)}</p>
        </>
      )}

      <div className="news-detail__primary-action">
        <button type="button" className="primary-button primary-button--wide" onClick={() => openExternalUrl(article.articleUrl)}>
          {messages.readFullArticle} <ExternalLinkIcon size={14} />
        </button>
        {articleDomain && <span className="news-detail__domain">{articleDomain}</span>}
      </div>

      <div className="news-detail__secondary-actions">
        <button type="button" className="secondary-button" onClick={copyArticleLink}>
          <LinkIcon size={14} /> <span key={isLinkCopied ? "copied" : "copy"} className="fade-in">{isLinkCopied ? messages.linkCopied : messages.copyLink}</span>
        </button>
        <button type="button" className="secondary-button" onClick={toggleReadState} aria-pressed={isUnread}>
          <UnreadDotIcon size={14} /> {isUnread ? messages.markAsRead : messages.markAsUnread}
        </button>
      </div>

      {(previousArticle || nextArticle) && (
        <nav className="article-pager" aria-label={`${messages.previousArticle} / ${messages.nextArticle}`}>
          {previousArticle ? (
            <button type="button" className="article-pager__link" onClick={() => onNavigate(previousArticle, "back")}>
              <span className="article-pager__direction">
                <BackIcon size={12} /> {messages.previousArticle}
              </span>
              <span className="article-pager__title">{previousArticle.title}</span>
            </button>
          ) : (
            <span />
          )}
          {nextArticle && (
            <button type="button" className="article-pager__link article-pager__link--next" onClick={() => onNavigate(nextArticle, "forward")}>
              <span className="article-pager__direction">
                {messages.nextArticle} <ChevronRightIcon size={12} />
              </span>
              <span className="article-pager__title">{nextArticle.title}</span>
            </button>
          )}
        </nav>
      )}
    </article>
  );
}

/** Cover image; without one (or when it fails to load) a cover in the source's color. */
function NewsHero({ article }: { article: Article }) {
  const [imageFailed, setImageFailed] = useState(false);
  if (article.imageUrl && !imageFailed) {
    return (
      <div className="news-hero">
        <FadeInImage className="news-hero__image" src={article.imageUrl} alt="" decoding="async" referrerPolicy="no-referrer" onLoadFailed={() => setImageFailed(true)} />
      </div>
    );
  }
  return (
    <div className="news-hero news-hero--placeholder" aria-hidden="true">
      <span className="news-hero__source">{article.sourceName}</span>
    </div>
  );
}
