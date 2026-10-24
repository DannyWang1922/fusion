// Runs inside the page after Readability has been loaded.
export function extractArticle() {
  const source = document.querySelector('main');
  if (!source) throw new Error('Article main element was not found');
  const clone = document.cloneNode(true);
  const main = clone.querySelector('main');
  // OpenAI places author metadata and recommendations after the complete body.
  const boundary = main.querySelector('[data-testid="author-list"]') ??
    [...main.querySelectorAll('h2')].find(node => node.textContent.trim() === 'Keep reading');
  if (boundary) {
    const range = clone.createRange();
    range.setStartBefore(boundary);
    range.setEnd(main, main.childNodes.length);
    range.deleteContents();
  }
  main.querySelectorAll('nav, aside, button, script, style, iframe, form, [role="navigation"], [data-testid*="table-of-contents"]').forEach(node => node.remove());
  main.querySelector('h1')?.remove();
  for (const node of main.querySelectorAll('img')) {
    const src = node.getAttribute('src');
    if (src) node.setAttribute('src', new URL(src, location.href).href);
    node.removeAttribute('srcset');
    node.removeAttribute('loading');
  }
  for (const node of main.querySelectorAll('a[href]')) {
    node.setAttribute('href', new URL(node.getAttribute('href'), location.href).href);
  }
  // OpenAI layout classes contain 'toc' on body sections, which otherwise
  // causes Readability to discard valid headings and illustrations.
  main.querySelectorAll('[class], [style]').forEach(node => { node.removeAttribute('class'); node.removeAttribute('style'); });
  const body = clone.createElement('body');
  body.append(main);
  clone.body.replaceWith(body);
  const result = new Readability(clone, { charThreshold: 100, keepClasses: false }).parse();
  if (!result || result.textContent.trim().length < 100) throw new Error('Article body was empty');
  return { title: result.title, content: result.content, text_length: result.textContent.length };
}
