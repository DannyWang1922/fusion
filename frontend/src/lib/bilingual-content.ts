import { processArticleContent } from "@/lib/content";
import type { TranslationSegment } from "@/lib/api";

const BLOCKS = "p,h1,h2,h3,h4,h5,h6,li,blockquote,figcaption,td,th,summary,div";

export type ReadingMode = "original" | "bilingual" | "chinese";

export function prepareBilingualContent(content: string, articleUrl?: string) {
  const html = processArticleContent(content, articleUrl);
  const container = document.createElement("div");
  container.innerHTML = html;
  let nodes = Array.from(container.querySelectorAll<HTMLElement>(BLOCKS)).filter(
    (node) => !node.closest("pre,code") && !node.querySelector(BLOCKS) && /[a-z]{2}/i.test(node.textContent ?? ""),
  );
  if (!nodes.length && !container.querySelector("pre,table") && /[a-z]{2}/i.test(container.textContent ?? "")) {
    const paragraph = document.createElement("p");
    paragraph.append(...Array.from(container.childNodes));
    container.append(paragraph);
    nodes = [paragraph];
  }
  const segments: TranslationSegment[] = nodes.map((node, index) => ({
    id: String(index), text: node.textContent ?? "", translation: "",
  }));
  return { html: container.innerHTML, segments };
}

export function renderBilingualContent(
  html: string, segments: TranslationSegment[], translated: TranslationSegment[], mode: ReadingMode,
) {
  if (mode === "original") return html;
  const container = document.createElement("div");
  container.innerHTML = html;
  const nodes = Array.from(container.querySelectorAll<HTMLElement>(BLOCKS)).filter(
    (node) => !node.closest("pre,code") && !node.querySelector(BLOCKS) && /[a-z]{2}/i.test(node.textContent ?? ""),
  );
  const translations = new Map(translated.map((segment) => [segment.id, segment]));
  nodes.forEach((node, index) => {
    const source = segments[index];
    const result = source ? translations.get(source.id) : undefined;
    if (!source || !result || result.text !== source.text || !result.translation) return;
    const original = document.createElement("span");
    original.className = "fusion-original";
    original.append(...Array.from(node.childNodes));
    if (mode === "chinese") original.hidden = true;
    const chinese = document.createElement("span");
    chinese.className = "fusion-translation";
    chinese.lang = "zh-CN";
    // Model output is text, never executable HTML.
    chinese.textContent = result.translation;
    node.append(original, chinese);
  });
  return container.innerHTML;
}
