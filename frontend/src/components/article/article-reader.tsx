import { useEffect, useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { LoaderCircle } from "lucide-react";
import { fullContentAPI, type Item, type ArticleFullContent } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { BilingualArticle } from "./bilingual-article";

export function ArticleReader({ article, articleUrl, fetchFullContent }: {
  article: Item;
  articleUrl?: string;
  fetchFullContent: boolean;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const queryKey = ["article-full-content", article.id];
  const enabled = fetchFullContent && article.id > 0 && !article.full_content_fetched_at;
  const started = useRef(false);
  const query = useQuery({
    queryKey,
    queryFn: async () => (await fullContentAPI.get(article.id)).data!,
    enabled,
    refetchIntervalInBackground: true,
    refetchInterval: (query) => query.state.data?.state === "running" ? 1500 : false,
  });
  const mutation = useMutation({
    mutationFn: () => fullContentAPI.start(article.id),
    onSuccess: (response) => {
      if (response.data) client.setQueryData<ArticleFullContent>(queryKey, response.data);
      void client.invalidateQueries({ queryKey });
    },
  });
  useEffect(() => {
    if (enabled && query.data?.state === "idle" && !query.data.item && !started.current) {
      started.current = true;
      mutation.mutate();
    }
  }, [enabled, query.data, mutation]);
  const current = query.data?.item ?? article;
  const ready = !fetchFullContent || !!current.full_content_fetched_at;
  const failed = mutation.isError || query.isError || query.data?.state === "error";
  return <>
    {fetchFullContent && <div className="mt-5 rounded-lg border bg-muted/30 px-3 py-2 text-xs text-muted-foreground" role="status">
      {ready ? t("fullContent.cached") : failed ? <div className="flex flex-wrap items-center gap-2">
        <span>{t("fullContent.failed")}</span>
        <Button size="sm" variant="outline" onClick={() => mutation.mutate()} disabled={mutation.isPending}>
          {t("fullContent.retry")}
        </Button>
      </div> : <span className="flex items-center gap-2">
        <LoaderCircle className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
        {t("fullContent.loading")}
      </span>}
    </div>}
    <BilingualArticle key={`${current.id}:${current.full_content_fetched_at ?? 0}`} article={current}
      articleUrl={articleUrl} allowTranslation={ready} />
  </>;
}
