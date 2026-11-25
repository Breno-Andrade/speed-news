package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"speed-news/apps/api/internal/domain"
)

// InsertArticles stores new articles and ignores ones already known.
// Articles published before autoReadBefore are stored as read, so the first run does not show 200 unread items.
func (store *Store) InsertArticles(ctx context.Context, articles []domain.Article, autoReadBefore time.Time) (int, error) {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = transaction.Rollback() }() // no-op after Commit

	// Known articles are kept as they are (read state included), except that a missing image is
	// filled in when a later fetch finds one (e.g. after improving image extraction).
	insertStatement, err := transaction.PrepareContext(ctx, `INSERT INTO articles
		(id, series_slug, source_slug, source_name, language_code, title, summary, article_url, image_url, published_at, fetched_at, read_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET image_url = excluded.image_url
		WHERE articles.image_url = '' AND excluded.image_url != ''`)
	if err != nil {
		return 0, err
	}
	defer insertStatement.Close()

	insertedCount := 0
	for _, article := range articles {
		var readAtValue any
		if article.PublishedAt.Before(autoReadBefore) {
			readAtValue = article.FetchedAt.Unix()
		}
		insertResult, err := insertStatement.ExecContext(ctx,
			article.ID, string(article.SeriesSlug), article.SourceSlug, article.SourceName, article.LanguageCode,
			article.Title, article.Summary, article.ArticleURL, article.ImageURL,
			article.PublishedAt.Unix(), article.FetchedAt.Unix(), readAtValue)
		if err != nil {
			return 0, fmt.Errorf("insert article %s: %w", article.ID, err)
		}
		affectedRows, _ := insertResult.RowsAffected()
		insertedCount += int(affectedRows)
	}
	return insertedCount, transaction.Commit()
}

// ArticleFilter narrows article queries. An empty LanguageCodes means every language.
type ArticleFilter struct {
	OnlyUnread    bool
	LanguageCodes []string
}

// whereClause builds "WHERE ..." with placeholders, or "" when nothing is filtered.
func (articleFilter ArticleFilter) whereClause() (string, []any) {
	conditions := []string{}
	queryArguments := []any{}
	if articleFilter.OnlyUnread {
		conditions = append(conditions, "read_at IS NULL")
	}
	if len(articleFilter.LanguageCodes) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(articleFilter.LanguageCodes)), ", ")
		conditions = append(conditions, "language_code IN ("+placeholders+")")
		for _, languageCode := range articleFilter.LanguageCodes {
			queryArguments = append(queryArguments, languageCode)
		}
	}
	if len(conditions) == 0 {
		return "", queryArguments
	}
	return " WHERE " + strings.Join(conditions, " AND "), queryArguments
}

// ListArticles returns the newest articles first.
func (store *Store) ListArticles(ctx context.Context, articleFilter ArticleFilter, maximumResults int) ([]domain.Article, error) {
	whereClause, queryArguments := articleFilter.whereClause()
	articlesQuery := `SELECT id, series_slug, source_slug, source_name, language_code, title, summary,
		article_url, image_url, published_at, fetched_at, read_at FROM articles` +
		whereClause + ` ORDER BY published_at DESC LIMIT ?`

	articleRows, err := store.database.QueryContext(ctx, articlesQuery, append(queryArguments, maximumResults)...)
	if err != nil {
		return nil, err
	}
	defer articleRows.Close()

	articles := []domain.Article{}
	for articleRows.Next() {
		var article domain.Article
		var seriesSlug string
		var publishedAtUnix, fetchedAtUnix int64
		var readAtUnix sql.NullInt64
		if err := articleRows.Scan(&article.ID, &seriesSlug, &article.SourceSlug, &article.SourceName,
			&article.LanguageCode, &article.Title, &article.Summary, &article.ArticleURL, &article.ImageURL,
			&publishedAtUnix, &fetchedAtUnix, &readAtUnix); err != nil {
			return nil, err
		}
		article.SeriesSlug = domain.SeriesSlug(seriesSlug)
		article.PublishedAt = time.Unix(publishedAtUnix, 0).UTC()
		article.FetchedAt = time.Unix(fetchedAtUnix, 0).UTC()
		if readAtUnix.Valid {
			readAt := time.Unix(readAtUnix.Int64, 0).UTC()
			article.ReadAt = &readAt
		}
		articles = append(articles, article)
	}
	return articles, articleRows.Err()
}

// CountUnreadArticles counts unread articles in the given languages (empty = all).
func (store *Store) CountUnreadArticles(ctx context.Context, languageCodes []string) (int, error) {
	whereClause, queryArguments := ArticleFilter{OnlyUnread: true, LanguageCodes: languageCodes}.whereClause()
	var unreadCount int
	err := store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM articles`+whereClause, queryArguments...).Scan(&unreadCount)
	return unreadCount, err
}

// MarkArticleRead returns false when the article does not exist.
func (store *Store) MarkArticleRead(ctx context.Context, articleID string, readAt time.Time) (bool, error) {
	updateResult, err := store.database.ExecContext(ctx,
		`UPDATE articles SET read_at = COALESCE(read_at, ?) WHERE id = ?`, readAt.Unix(), articleID)
	if err != nil {
		return false, err
	}
	affectedRows, err := updateResult.RowsAffected()
	return affectedRows > 0, err
}

// ArticlePage identifies an article whose page should be checked for a share image.
type ArticlePage struct {
	ArticleID  string
	ArticleURL string
}

// ListArticlesAwaitingImageLookup returns the newest articles of a source with no image that
// were never checked.
func (store *Store) ListArticlesAwaitingImageLookup(ctx context.Context, sourceSlug string, maximumResults int) ([]ArticlePage, error) {
	articleRows, err := store.database.QueryContext(ctx, `SELECT id, article_url FROM articles
		WHERE source_slug = ? AND image_url = '' AND image_lookup_done = 0
		ORDER BY published_at DESC LIMIT ?`, sourceSlug, maximumResults)
	if err != nil {
		return nil, err
	}
	defer articleRows.Close()

	articlePages := []ArticlePage{}
	for articleRows.Next() {
		var articlePage ArticlePage
		if err := articleRows.Scan(&articlePage.ArticleID, &articlePage.ArticleURL); err != nil {
			return nil, err
		}
		articlePages = append(articlePages, articlePage)
	}
	return articlePages, articleRows.Err()
}

// SaveImageLookupResult stores the found image (if any) and marks the article as checked.
func (store *Store) SaveImageLookupResult(ctx context.Context, articleID string, imageURL string) error {
	_, err := store.database.ExecContext(ctx, `UPDATE articles
		SET image_url = CASE WHEN image_url = '' THEN ? ELSE image_url END, image_lookup_done = 1
		WHERE id = ?`, imageURL, articleID)
	return err
}

// MarkArticleUnread returns false when the article does not exist.
func (store *Store) MarkArticleUnread(ctx context.Context, articleID string) (bool, error) {
	updateResult, err := store.database.ExecContext(ctx, `UPDATE articles SET read_at = NULL WHERE id = ?`, articleID)
	if err != nil {
		return false, err
	}
	affectedRows, err := updateResult.RowsAffected()
	return affectedRows > 0, err
}

// MarkAllArticlesRead only touches the languages the user sees, so hidden languages keep their state.
func (store *Store) MarkAllArticlesRead(ctx context.Context, readAt time.Time, languageCodes []string) error {
	whereClause, queryArguments := ArticleFilter{OnlyUnread: true, LanguageCodes: languageCodes}.whereClause()
	_, err := store.database.ExecContext(ctx, `UPDATE articles SET read_at = ?`+whereClause,
		append([]any{readAt.Unix()}, queryArguments...)...)
	return err
}

// DeleteArticlesPublishedBefore keeps the database small.
func (store *Store) DeleteArticlesPublishedBefore(ctx context.Context, cutoff time.Time) error {
	_, err := store.database.ExecContext(ctx, `DELETE FROM articles WHERE published_at < ?`, cutoff.Unix())
	return err
}
