import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Languages, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { APIError, translationAPI, type Item, type ArticleTranslation } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { prepareBilingualContent, renderBilingualContent, type ReadingMode } from "@/lib/bilingual-content";
import "./bilingual-article.css";

export function BilingualArticle({ article, articleUrl, allowTranslation = true }: { article: Item; articleUrl?: string; allowTranslation?: boolean }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [mode, setMode] = useState<ReadingMode>("original");
  const queryKey = ["article-translation", article.id, article.full_content_fetched_at ?? 0];
  const prepared = useMemo(() => prepareBilingualContent(article.content, articleUrl), [article.content, articleUrl]);
  const query = useQuery({
    queryKey,
    queryFn: async () => (await translationAPI.get(article.id)).data!,
    enabled: allowTranslation && article.id > 0,
    refetchInterval: (query) => query.state.data?.state === "running" ? 1500 : false,
  });
  const mutation = useMutation({
    mutationFn: () => translationAPI.start(article.id, prepared.segments),
    onSuccess: (response) => {
      if (response.data) client.setQueryData<ArticleTranslation>(queryKey, response.data);
      void client.invalidateQueries({ queryKey });
    },
  });
  const translated = query.data?.segments ?? [];
  const completed = prepared.segments.filter((source) => translated.some(
    (result) => result.id === source.id && result.text === source.text && result.translation,
  )).length;
  const complete = prepared.segments.length > 0 && completed === prepared.segments.length;
  const running = mutation.isPending || query.data?.state === "running";
  const error = mutation.error instanceof APIError ? mutation.error.message
    : mutation.error || query.isError ? "translation_failed" : query.data?.error;
  const errorText = error === "translation_busy" ? t("translation.busy")
    : error === "translation_unavailable" ? t("translation.unavailable") : t("translation.failed");
  const content = useMemo(() => renderBilingualContent(
    prepared.html, prepared.segments, translated, mode,
  ), [prepared, translated, mode]);

  const openBilingual = () => {
    setMode("bilingual");
    if (!complete && !running && !query.isLoading) mutation.mutate();
  };

  return (
    <div className="mt-6 min-w-0">
      {allowTranslation && article.id > 0 && prepared.segments.length > 0 && (
        <div className="mb-6 rounded-lg border bg-muted/30 px-3 py-3">
          <div className="flex flex-wrap items-center gap-2" aria-label={t("translation.readingMode")}>
            <Languages className="mr-1 h-4 w-4 text-muted-foreground" aria-hidden="true" />
            <Button size="sm" variant={mode === "original" ? "secondary" : "ghost"}
              aria-pressed={mode === "original"} onClick={() => setMode("original")}>
              {t("translation.original")}
            </Button>
            <Button size="sm" variant={mode === "bilingual" ? "secondary" : "ghost"}
              aria-pressed={mode === "bilingual"} disabled={query.isLoading}
              onClick={openBilingual}>
              {running && <LoaderCircle className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />}
              {t("translation.bilingual")}
            </Button>
            <Button size="sm" variant={mode === "chinese" ? "secondary" : "ghost"}
              aria-pressed={mode === "chinese"} disabled={!complete}
              onClick={() => setMode("chinese")}>
              {t("translation.chinese")}
            </Button>
          </div>
          <p role="status" className="mt-2 text-xs leading-relaxed text-muted-foreground">
            {running ? t("translation.progress", { done: completed, total: prepared.segments.length })
              : complete ? t("translation.cached") : t("translation.hint")}
          </p>
          {error && !running && (
            <div role="alert" className="mt-2 flex flex-wrap items-center gap-2 text-xs text-destructive">
              <span>{errorText}</span>
              <Button size="sm" variant="outline" onClick={() => { setMode("bilingual"); mutation.mutate(); }}>
                {t("translation.retry")}
              </Button>
            </div>
          )}
        </div>
      )}
      <div className="typeset typeset-article min-w-0 max-w-none fusion-bilingual-content"
        dangerouslySetInnerHTML={{ __html: content }} />
    </div>
  );
}
