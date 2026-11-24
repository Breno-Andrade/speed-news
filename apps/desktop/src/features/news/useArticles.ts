import { useCallback, useEffect, useState } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import type { ArticleStatusFilter } from "../../shared/api/apiClient";
import type { Article } from "../../shared/api/types";
import { useSettings } from "../settings/SettingsProvider";

type ArticlesState = { status: "loading" } | { status: "loaded"; articles: Article[] } | { status: "failed" };

/**
 * Reloads whenever the filter, the unread count (new articles arrived or something was read)
 * or the chosen news languages change — the count can stay the same while the list changes.
 */
export function useArticles(statusFilter: ArticleStatusFilter, unreadCount: number) {
  const apiClient = useApiClient();
  const { userSettings } = useSettings();
  const newsLanguagesKey = userSettings.newsLanguageCodes.join(",");
  const [articlesState, setArticlesState] = useState<ArticlesState>({ status: "loading" });
  const [reloadSignal, setReloadSignal] = useState(0);

  useEffect(() => {
    let isCancelled = false;
    apiClient
      .listArticles(statusFilter)
      .then((articles) => {
        if (!isCancelled) setArticlesState({ status: "loaded", articles });
      })
      .catch(() => {
        if (!isCancelled) setArticlesState({ status: "failed" });
      });
    return () => {
      isCancelled = true;
    };
  }, [apiClient, statusFilter, unreadCount, reloadSignal, newsLanguagesKey]);

  const reloadArticles = useCallback(() => setReloadSignal((previousSignal) => previousSignal + 1), []);

  /** Optimistic update: the dot disappears immediately, the server confirms in the background. */
  const markArticleRead = useCallback(
    async (articleId: string) => {
      setArticlesState((previousState) =>
        previousState.status === "loaded"
          ? {
              status: "loaded",
              articles: previousState.articles.map((article) =>
                article.id === articleId ? { ...article, readAt: article.readAt ?? new Date().toISOString() } : article,
              ),
            }
          : previousState,
      );
      await apiClient.markArticleRead(articleId);
    },
    [apiClient],
  );

  const markArticleUnread = useCallback(
    async (articleId: string) => {
      setArticlesState((previousState) =>
        previousState.status === "loaded"
          ? {
              status: "loaded",
              articles: previousState.articles.map((article) => (article.id === articleId ? { ...article, readAt: undefined } : article)),
            }
          : previousState,
      );
      await apiClient.markArticleUnread(articleId);
    },
    [apiClient],
  );

  return { articlesState, reloadArticles, markArticleRead, markArticleUnread };
}
