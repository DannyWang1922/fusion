import {
  useNavigate,
  useParams,
  useSearch,
} from "@tanstack/react-router";
import { useCallback } from "react";
import {
  defaultArticleFilter,
  isArticleFilter,
  type ArticleFilter,
} from "@/lib/article-filter";
import { parsePositiveIntegerParam } from "@/lib/route-params";

import type { ArticlePeriod } from "@/lib/api/types";

export type { ArticleFilter } from "@/lib/article-filter";

export function useUrlState() {
  const navigate = useNavigate();
  const params = useParams({ strict: false }) as {
    filter?: string;
    feedId?: string;
    groupId?: string;
  };
  const search = useSearch({ strict: false }) as Record<string, unknown>;

  const routeFeedId = parsePositiveIntegerParam(params.feedId);
  const routeGroupId = parsePositiveIntegerParam(params.groupId);
  const selectedArticleId = parsePositiveIntegerParam(search.article);
  const routeFilter =
    typeof params.filter === "string" && isArticleFilter(params.filter)
      ? params.filter
      : defaultArticleFilter;

  const selectedFeedId = routeFeedId;
  const selectedGroupId = routeGroupId;
  const articleFilter = routeFilter;
  const articlePeriod: ArticlePeriod = search.period === "older" ? "older" : "recent";

  const navigateToList = useCallback(
    ({
      filter,
      period,
      feedId,
      groupId,
      articleId,
      replace,
    }: {
      filter?: ArticleFilter;
      period?: ArticlePeriod;
      feedId?: number | null;
      groupId?: number | null;
      articleId?: number | null;
      replace?: boolean;
    } = {}) => {
      const nextFilter = filter ?? articleFilter;
      const nextFeedId = feedId === undefined ? selectedFeedId : feedId;
      const nextGroupId = groupId === undefined ? selectedGroupId : groupId;
      const nextArticleId = articleId === undefined ? selectedArticleId : articleId;
      const nextPeriod = period ?? articlePeriod;
      const nextSearch = { article: nextArticleId ?? undefined, period: nextPeriod === "older" ? "older" : undefined };

      if (nextGroupId !== null) {
        navigate({
          to: "/groups/$groupId/$filter",
          params: {
            groupId: String(nextGroupId),
            filter: nextFilter,
          },
          search: nextSearch,
          replace: replace ?? true,
        });
        return;
      }

      if (nextFeedId !== null) {
        navigate({
          to: "/feeds/$feedId/$filter",
          params: {
            feedId: String(nextFeedId),
            filter: nextFilter,
          },
          search: nextSearch,
          replace: replace ?? true,
        });
        return;
      }

      navigate({
        to: "/$filter",
        params: { filter: nextFilter },
        search: nextSearch,
        replace: replace ?? true,
      });
    },
    [
      articleFilter,
      articlePeriod,
      navigate,
      selectedArticleId,
      selectedFeedId,
      selectedGroupId,
    ],
  );

  const setSelectedFeed = useCallback(
    (feedId: number | null) => {
      navigateToList({
        feedId,
        groupId: null,
        articleId: null,
      });
    },
    [navigateToList],
  );

  const setSelectedGroup = useCallback(
    (groupId: number | null) => {
      navigateToList({
        groupId,
        feedId: null,
        articleId: null,
      });
    },
    [navigateToList],
  );

  const setSelectedArticle = useCallback(
    (articleId: number | null, period?: ArticlePeriod) => {
      if (articleId === null) {
        navigateToList({ articleId: null, replace: true });
        return;
      }

      navigateToList({
        articleId,
        period,
        replace: selectedArticleId !== null,
      });
    },
    [navigateToList, selectedArticleId],
  );

  const setArticleFilter = useCallback(
    (filter: ArticleFilter) => {
      navigateToList({
        filter,
        articleId: null,
      });
    },
    [navigateToList],
  );

  const setArticlePeriod = useCallback((period: ArticlePeriod) => {
    navigateToList({ period, articleId: null });
  }, [navigateToList]);

  const selectTopLevelFilter = useCallback(
    (filter: ArticleFilter) => {
      navigateToList({
        filter,
        feedId: null,
        groupId: null,
        articleId: null,
      });
    },
    [navigateToList],
  );

  const selectAll = useCallback(() => {
    selectTopLevelFilter("all");
  }, [selectTopLevelFilter]);

  return {
    selectedFeedId,
    selectedGroupId,
    selectedArticleId,
    articleFilter,
    articlePeriod,
    setArticlePeriod,
    setSelectedFeed,
    setSelectedGroup,
    setSelectedArticle,
    setArticleFilter,
    selectTopLevelFilter,
    selectAll,
  };
}
