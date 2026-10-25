import { useLocation } from "@tanstack/react-router";
import { Inbox, Layers, Star } from "lucide-react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { isArticleFilter } from "@/lib/article-filter";
import { useGroups } from "@/queries/groups";
import { useFeedLookup, useUnreadCounts } from "@/queries/feeds";
import { useBookmarkLookup } from "@/queries/bookmarks";
import { useUrlState } from "@/hooks/use-url-state";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { FeedGroup } from "./feed-group";
import { FeedItem } from "./feed-item";

export function FeedList() {
  const { t } = useI18n();
  const { data: groups = [], isLoading } = useGroups();
  const { total: starredTotal } = useBookmarkLookup();
  const {
    selectedFeedId,
    selectedGroupId,
    articleFilter,
    articlePeriod,
    setArticlePeriod,
    selectTopLevelFilter,
  } = useUrlState();
  const { feeds, getFeedsByGroup } = useFeedLookup(articlePeriod);
  const { getTotalUnreadCount } = useUnreadCounts(articlePeriod);
  const { pathname } = useLocation();
  const firstPathSegment = pathname.split("/").filter(Boolean)[0];
  const isOnHomePage =
    typeof firstPathSegment === "string" && isArticleFilter(firstPathSegment);
  const isTopLevelSelected =
    isOnHomePage && selectedFeedId === null && selectedGroupId === null;
  const totalUnread = getTotalUnreadCount();
  const starredCount = starredTotal;

  const topFilters: Array<{
    value: "all" | "unread" | "starred";
    label: string;
    count?: number;
    icon: typeof Inbox;
  }> = [
    {
      value: "unread",
      label: t("article.filter.unread"),
      count: totalUnread,
      icon: Inbox,
    },
    {
      value: "starred",
      label: t("article.filter.starred"),
      count: starredCount,
      icon: Star,
    },
    {
      value: "all",
      label: t("article.filter.all"),
      icon: Layers,
    },
  ];

  if (isLoading && groups.length === 0) {
    return (
      <div className="flex-1 p-4">
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-8 animate-pulse rounded-md bg-accent" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <ScrollArea className="min-h-0 flex-1 w-full min-w-0 overflow-hidden [&_[data-slot=scroll-area-viewport]>div]:!block">
      <div className="w-full min-w-0 p-2 space-y-0.5">
        {/* Top-level filters */}
        <div className="space-y-0.5">
          {topFilters.map(({ value, label, count, icon: Icon }) => (
            <button
              key={value}
              onClick={() => selectTopLevelFilter(value)}
              className={cn(
                "flex w-full min-w-0 items-center gap-1.5 rounded-md px-2 py-1 text-left text-sm transition-colors",
                isTopLevelSelected && articleFilter === value
                  ? "bg-accent text-accent-foreground"
                  : "hover:bg-accent/50",
              )}
            >
              <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1">{label}</span>
              {typeof count === "number" && (
                <span className="shrink-0 text-[11px] text-muted-foreground">
                  {count}
                </span>
              )}
            </button>
          ))}
        </div>

        {/* Feeds header */}
        <div className="mt-2 flex items-center justify-between px-2 py-1">
          <span className="text-[11px] font-medium text-muted-foreground">
            {t("search.group.feeds")}
          </span>
        </div>

        <fieldset disabled={articleFilter === "starred"} className="mx-2 mb-2 min-w-0">
          <legend className="sr-only">{t("sidebar.period.label")}</legend>
          <div role="group" aria-label={t("sidebar.period.label")} className="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1">
            {(["recent", "older"] as const).map((period) => (
              <button
                key={period}
                type="button"
                aria-pressed={articlePeriod === period}
                onClick={() => setArticlePeriod(period)}
                className={cn(
                  "min-w-0 rounded-md px-2 py-1.5 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-default disabled:opacity-50",
                  articlePeriod === period
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {t(period === "recent" ? "article.period.recent" : "article.period.olderShort")}
              </button>
            ))}
          </div>
          {articleFilter === "starred" && (
            <p className="mt-1.5 text-[11px] leading-relaxed text-muted-foreground">
              {t("article.period.bookmarks")}
            </p>
          )}
        </fieldset>

        {/* Feed groups */}
        <div className="w-full min-w-0 space-y-0.5">
          {groups.map((group) => {
            const groupFeeds = getFeedsByGroup(group.id);

            return (
              <FeedGroup
                key={group.id}
                groupId={group.id}
                name={group.name}
                feeds={groupFeeds}
              />
            );
          })}

          {/* Ungrouped feeds (group_id = 0) */}
          {feeds
            .filter((f) => f.group_id === 0)
            .map((feed) => (
              <FeedItem key={feed.id} feed={feed} />
            ))}
        </div>
      </div>
    </ScrollArea>
  );
}
