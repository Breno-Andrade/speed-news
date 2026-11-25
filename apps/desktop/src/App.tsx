import { useEffect, useState, type AnimationEvent } from "react";
import { NewsDetail, type ArticleNavigationDirection } from "./features/news/NewsDetail";
import { NewsList } from "./features/news/NewsList";
import { NewsListSkeleton } from "./features/news/NewsListSkeleton";
import { useArticles } from "./features/news/useArticles";
import { useUnreadCount } from "./features/news/useUnreadCount";
import { RaceWeekendCard } from "./features/race-weekend/RaceWeekendCard";
import { SettingsPanel } from "./features/settings/SettingsPanel";
import { UpdateNotices } from "./features/updates/UpdateNotices";
import { useApiClient } from "./shared/api/ApiProvider";
import type { ArticleStatusFilter } from "./shared/api/apiClient";
import type { Article } from "./shared/api/types";
import { useLocale } from "./shared/i18n/LocaleProvider";
import { hideMainWindow, isRunningInTauri, onPopoverShown, quitApp } from "./shared/platform/tauriBridge";
import { useDelayedFlag } from "./shared/ui/Skeleton";
import { BackIcon, CheckAllIcon, PowerIcon, RefreshIcon, SettingsIcon } from "./shared/ui/icons";

type ArticleView = {
  viewName: "article";
  article: Article;
  /** The list as it was when the article was opened: "previous/next" must not shift while the
   *  articles being read drop out of the "unread" list. */
  siblingArticles: Article[];
  direction: ArticleNavigationDirection;
};

type CurrentView = { viewName: "home" } | ArticleView | { viewName: "settings" };

export function App() {
  const { messages } = useLocale();
  const apiClient = useApiClient();
  const unreadCount = useUnreadCount();
  const [currentView, setCurrentView] = useState<CurrentView>({ viewName: "home" });
  const [isHomeReturning, setIsHomeReturning] = useState(false);
  const [statusFilter, setStatusFilter] = useState<ArticleStatusFilter>("unread");
  const [isRefreshing, setIsRefreshing] = useState(false);
  const { articlesState, reloadArticles, markArticleRead, markArticleUnread } = useArticles(statusFilter, unreadCount);
  const showsNewsSkeleton = useDelayedFlag(articlesState.status === "loading");
  const isPopoverEntering = usePopoverEntrance();

  // Esc hides the popover, whatever the "close when clicking outside" setting is.
  useEffect(() => {
    const hideOnEscape = (keyboardEvent: KeyboardEvent) => {
      if (keyboardEvent.key === "Escape") hideMainWindow().catch(() => undefined);
    };
    window.addEventListener("keydown", hideOnEscape);
    return () => window.removeEventListener("keydown", hideOnEscape);
  }, []);

  function openArticle(article: Article) {
    const siblingArticles = articlesState.status === "loaded" ? articlesState.articles : [article];
    setCurrentView({ viewName: "article", article, siblingArticles, direction: "forward" });
    if (!article.readAt) markArticleRead(article.id).catch(() => undefined);
  }

  function navigateToArticle(targetArticle: Article, direction: ArticleNavigationDirection) {
    setCurrentView((previousView) =>
      previousView.viewName === "article" ? { ...previousView, article: targetArticle, direction } : previousView,
    );
    markArticleRead(targetArticle.id).catch(() => undefined);
  }

  async function refreshArticles() {
    setIsRefreshing(true);
    await apiClient.requestRefresh().catch(() => undefined);
    // The backend refreshes asynchronously; new articles arrive through the unread-count stream.
    window.setTimeout(() => {
      reloadArticles();
      setIsRefreshing(false);
    }, 2500);
  }

  function goHome() {
    setCurrentView({ viewName: "home" });
    setIsHomeReturning(true);
  }

  // Only the view's own animation ends the "returning" state, not the ones of its children.
  function finishHomeReturn(animationEvent: AnimationEvent<HTMLDivElement>) {
    if (animationEvent.target === animationEvent.currentTarget) setIsHomeReturning(false);
  }

  const isHome = currentView.viewName === "home";
  const articleNeighbors = currentView.viewName === "article" ? findNeighbors(currentView) : {};

  return (
    <div className={`app-shell ${isPopoverEntering ? "app-shell--entering" : ""}`}>
      <header className="app-header">
        <div key={isHome ? "title" : "back"} className="fade-in">
          {isHome ? (
            <h1 className="app-header__title">
              {messages.appTitle}
              {unreadCount > 0 && (
                // key restarts the pop animation whenever the number changes
                <span key={unreadCount} className="unread-pill unread-pill--pop">
                  {unreadCount > 99 ? "99+" : unreadCount}
                </span>
              )}
            </h1>
          ) : (
            <button type="button" className="icon-button icon-button--labeled" onClick={goHome}>
              <BackIcon /> {messages.back}
            </button>
          )}
        </div>
        {isHome && (
          <div className="app-header__actions fade-in">
            <button type="button" className="icon-button" title={messages.refresh} aria-label={messages.refresh} onClick={refreshArticles} disabled={isRefreshing}>
              <span className={isRefreshing ? "icon-slot spin" : "icon-slot"}>
                <RefreshIcon />
              </span>
            </button>
            <button type="button" className="icon-button" title={messages.settings} aria-label={messages.settings} onClick={() => setCurrentView({ viewName: "settings" })}>
              <SettingsIcon />
            </button>
            {isRunningInTauri() && (
              <button type="button" className="icon-button" title={messages.trayQuitMenuLabel} aria-label={messages.trayQuitMenuLabel} onClick={() => quitApp()}>
                <PowerIcon />
              </button>
            )}
          </div>
        )}
      </header>

      <UpdateNotices />

      <main className="app-content">
        {/* Home stays mounted while another view is open: coming back keeps scroll and data. */}
        <div className={`app-view ${isHomeReturning ? "view-enter-back" : ""}`} hidden={!isHome} onAnimationEnd={finishHomeReturn}>
          <RaceWeekendCard onChangeTimeZone={() => setCurrentView({ viewName: "settings" })} />

          <div className="news-toolbar">
            <div className="segmented-control" role="tablist">
              {(["unread", "all"] as const).map((filterOption) => (
                <button
                  key={filterOption}
                  type="button"
                  role="tab"
                  aria-selected={statusFilter === filterOption}
                  className="segmented-control__option"
                  onClick={() => setStatusFilter(filterOption)}
                >
                  {filterOption === "unread" ? messages.unreadFilter : messages.allFilter}
                </button>
              ))}
            </div>
            <button
              type="button"
              className={`icon-button icon-button--labeled mark-all-button ${unreadCount > 0 ? "" : "mark-all-button--hidden"}`}
              onClick={() => apiClient.markAllArticlesRead()}
              disabled={unreadCount === 0}
              aria-hidden={unreadCount === 0}
            >
              <CheckAllIcon size={14} /> {messages.markAllRead}
            </button>
          </div>

          {articlesState.status === "loading" && (showsNewsSkeleton ? <NewsListSkeleton label={messages.loadingArticles} /> : <div className="news-pending" />)}
          {articlesState.status === "failed" && <p className="empty-state fade-in">{messages.loadFailed}</p>}
          {articlesState.status === "loaded" && (
            <NewsList
              articles={articlesState.articles}
              emptyMessage={statusFilter === "unread" ? messages.noUnreadArticles : messages.noArticles}
              onOpenArticle={openArticle}
            />
          )}
        </div>

        {currentView.viewName === "article" && (
          <div key={`article-${currentView.article.id}`} className={`app-view ${currentView.direction === "back" ? "view-enter-back" : "view-enter-forward"}`}>
            <NewsDetail
              article={currentView.article}
              previousArticle={articleNeighbors.previousArticle}
              nextArticle={articleNeighbors.nextArticle}
              onNavigate={navigateToArticle}
              onMarkRead={markArticleRead}
              onMarkUnread={markArticleUnread}
            />
          </div>
        )}
        {currentView.viewName === "settings" && (
          <div key="settings" className="app-view view-enter-forward">
            <SettingsPanel />
          </div>
        )}
      </main>
    </div>
  );
}

function findNeighbors(articleView: ArticleView): { previousArticle?: Article; nextArticle?: Article } {
  const articleIndex = articleView.siblingArticles.findIndex((siblingArticle) => siblingArticle.id === articleView.article.id);
  if (articleIndex < 0) return {};
  return {
    previousArticle: articleView.siblingArticles[articleIndex - 1],
    nextArticle: articleView.siblingArticles[articleIndex + 1],
  };
}

/** True for a moment each time the native shell shows the popover, to play the entrance. */
function usePopoverEntrance(): boolean {
  const [isEntering, setIsEntering] = useState(false);
  useEffect(() => {
    let exitTimer: number | undefined;
    const stopListening = onPopoverShown(() => {
      setIsEntering(true);
      window.clearTimeout(exitTimer);
      exitTimer = window.setTimeout(() => setIsEntering(false), 260);
    });
    return () => {
      window.clearTimeout(exitTimer);
      stopListening();
    };
  }, []);
  return isEntering;
}
