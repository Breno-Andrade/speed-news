import type { Article } from "../../shared/api/types";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { formatRelativeTime } from "../../shared/time/timeZone";
import { FadeInImage } from "../../shared/ui/FadeInImage";

/** Entrance stagger per item, capped so a long list never feels slow to appear. */
const STAGGER_STEP_MS = 30;
const MAXIMUM_STAGGER_MS = 240;

type NewsListProps = {
  articles: Article[];
  emptyMessage: string;
  onOpenArticle: (article: Article) => void;
};

export function NewsList({ articles, emptyMessage, onOpenArticle }: NewsListProps) {
  const { locale } = useLocale();

  if (articles.length === 0) {
    return <p className="empty-state fade-in">{emptyMessage}</p>;
  }

  // Items are keyed by id: on a reload only articles that just arrived mount, so only they animate.
  return (
    <ul className="news-list">
      {articles.map((article, articleIndex) => {
        const isUnread = !article.readAt;
        return (
          <li key={article.id} className="news-list__entry" style={{ animationDelay: `${Math.min(articleIndex * STAGGER_STEP_MS, MAXIMUM_STAGGER_MS)}ms` }}>
            <button type="button" className={`news-item ${isUnread ? "news-item--unread" : ""}`} onClick={() => onOpenArticle(article)}>
              <span className="news-item__dot" aria-hidden="true" />
              <span className="news-item__text">
                <span className="news-item__title">{article.title}</span>
                <span className="news-item__meta">
                  {article.sourceName} · {formatRelativeTime(article.publishedAt, locale)}
                  {article.languageCode !== locale.slice(0, 2) && <span className="news-item__language">{article.languageCode.toUpperCase()}</span>}
                </span>
              </span>
              {article.imageUrl && <FadeInImage className="news-item__thumbnail" src={article.imageUrl} alt="" loading="lazy" decoding="async" referrerPolicy="no-referrer" />}
            </button>
          </li>
        );
      })}
    </ul>
  );
}
