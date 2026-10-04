import { useEffect, useRef, useState } from "react";
import {
  getDocument,
  GlobalWorkerOptions,
  type PDFDocumentProxy,
  type RenderTask,
} from "pdfjs-dist";
import worker from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import type { Variant } from "./App";
GlobalWorkerOptions.workerSrc = worker;
export default function Reader({
  variant,
  initialPage,
}: {
  variant: Variant;
  initialPage: number;
}) {
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<{ page: number; snippet: string }[] | null>(
    null,
  );
  const [searchError, setSearchError] = useState("");
  const [pdf, setPDF] = useState<PDFDocumentProxy | null>(null);
  const [page, setPage] = useState(
    Number.isInteger(initialPage) && initialPage > 0 ? initialPage : 1,
  );
  const [scale, setScale] = useState(1);
  const [error, setError] = useState("");
  const [text, setText] = useState("");
  const [loading, setLoading] = useState(true);
  const canvas = useRef<HTMLCanvasElement>(null);
  const wrap = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(700);
  useEffect(() => {
    const node = wrap.current;
    if (!node) return;
    const observer = new ResizeObserver((entries) =>
      setWidth(entries[0].contentRect.width),
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    setLoading(true);
    setError("");
    setPDF(null);
    let active = true;
    const task = getDocument({
      url: `/api/assets/${variant.id}`,
      cMapUrl: "/pdfjs/cmaps/",
      cMapPacked: true,
      standardFontDataUrl: "/pdfjs/standard_fonts/",
      wasmUrl: "/pdfjs/wasm/",
      // Bound decoded scan layers; JPEG 2000 can otherwise allocate far more
      // memory than the fit-width canvas for large, compressed source pages.
      canvasMaxAreaInBytes: 32 * 1024 * 1024,
      isEvalSupported: false,
      enableXfa: false,
    });
    task.promise
      .then((doc) => {
        if (active) {
          setPDF(doc);
          setPage((p) => Math.min(p, doc.numPages));
        }
      })
      .catch(() => {
        if (active) {
          setError(
            "The PDF could not be opened. Try downloading the original.",
          );
          setLoading(false);
        }
      });
    return () => {
      active = false;
      void task.destroy();
    };
  }, [variant.id]);
  useEffect(() => {
    if (!pdf || !canvas.current) return;
    let cancelled = false;
    let render: RenderTask | undefined;
    setLoading(true);
    async function draw() {
      const p = await pdf!.getPage(page);
      if (cancelled || !canvas.current) return;
      const original = p.getViewport({ scale: 1 });
      const viewport = p.getViewport({
        scale: Math.min(width / original.width, 1.5) * scale,
      });
      const c = canvas.current;
      c.width = Math.round(viewport.width);
      c.height = Math.round(viewport.height);
      render = p.render({ canvas: c, viewport });
      await render.promise;
      const content = await p.getTextContent();
      if (!cancelled) {
        setText(content.items.map((i) => ("str" in i ? i.str : "")).join(" "));
        setLoading(false);
      }
    }
    draw().catch(() => {
      if (!cancelled) {
        setError("This page could not be rendered.");
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
      render?.cancel();
    };
  }, [pdf, page, width, scale]);
  const go = (n: number) => {
    if (pdf && Number.isInteger(n) && n >= 1 && n <= pdf.numPages && n !== page) {
      setLoading(true);
      setPage(n);
      setError("");
      const q = new URLSearchParams(location.search);
      q.set("page", String(n));
      history.replaceState(null, "", `?${q}`);
    }
  };
  async function search(e: React.FormEvent) {
    e.preventDefault();
    setSearchError("");
    setHits(null);
    try {
      const r = await fetch(
        `/api/assets/${variant.id}/search?q=${encodeURIComponent(query)}`,
      );
      if (!r.ok) throw new Error();
      setHits(await r.json());
    } catch {
      setSearchError("Document search is unavailable. Please try again.");
    }
  }
  return (
    <section className="reader" aria-label="Document reader">
      <div className="reader-controls">
        <button onClick={() => go(page - 1)} disabled={page <= 1}>
          Previous page
        </button>
        <label>
          Page
          <input
            aria-label="Page number"
            type="number"
            min={1}
            max={pdf?.numPages || 1}
            value={page}
            onChange={(e) => go(Number(e.target.value))}
          />
        </label>
        <span>of {pdf?.numPages || "…"}</span>
        <button
          onClick={() => go(page + 1)}
          disabled={!pdf || page >= pdf.numPages}
        >
          Next page
        </button>
        <label>
          Zoom
          <select
            aria-label="Zoom"
            value={scale}
            onChange={(e) => {
              setLoading(true);
              setScale(Number(e.target.value));
            }}
          >
            <option value={1}>Fit width</option>
            <option value={1.25}>125%</option>
            <option value={1.5}>150%</option>
            <option value={2}>200%</option>
          </select>
        </label>
      </div>
      {(variant.text_status === "page-text" ||
        variant.text_status === "document-text") && (
        <form onSubmit={search} className="search">
          <label>
            Search this document
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              maxLength={200}
              required
            />
          </label>
          <button>Find</button>
        </form>
      )}
      {searchError && <p role="alert">{searchError}</p>}
      {hits && (
        <div aria-live="polite">
          <p>{hits.length} matching pages or text sections</p>
          {hits.map((h, i) => (
            <p key={i}>
              {h.page > 0 ? (
                <button onClick={() => go(h.page)}>Page {h.page}</button>
              ) : (
                "Document text; page mapping unavailable"
              )}{" "}
              {h.snippet}
            </p>
          ))}
        </div>
      )}
      {loading && <p role="status">Loading page…</p>}
      {error && <p role="alert">{error}</p>}
      <div className="canvas-wrap" ref={wrap}>
        <canvas
          ref={canvas}
          aria-label={`PDF page ${page}`}
          data-testid="pdf-canvas"
        />
      </div>
      <details>
        <summary>Readable text for this page</summary>
        <p>{text || "No text layer is available on this page."}</p>
      </details>
    </section>
  );
}
