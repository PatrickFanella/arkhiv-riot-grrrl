import { lazy, Suspense, useEffect, useState } from "react";
import Catalog from "./Catalog";
import {
  FavoriteButton,
  FavoriteShelf,
  ShareControls,
  useBrowserFavorites,
} from "./Preferences";
const publicURL =
  import.meta.env.VITE_PUBLIC_URL ||
  globalThis.location?.origin ||
  "http://localhost";
const contact = import.meta.env.VITE_CORRECTION_CONTACT || "info@subcult.tv";
const Reader = lazy(() => import("./Reader"));
export interface Variant {
  public_approved?: boolean;
  id: string;
  filename: string;
  sha256: string;
  size: number;
  pages: number;
  text_status: string;
}
export interface DocumentRecord {
  id: string;
  collection: string;
  source_item: string;
  original_date?: string;
  metadata: {
    title: string;
    creator?: string;
    date?: string;
    language?: string;
    subjects?: string[];
  };
  rights: string;
  variants: Variant[];
  hits: { page: number; variant: string; snippet: string }[];
}
export interface Collection {
  id: string;
  name: string;
  introduction: string;
  accent: string;
}
export interface Listing {
  documents: DocumentRecord[];
  total: number;
  limit: number;
  offset: number;
}
async function json<T>(url: string, signal: AbortSignal): Promise<T> {
  const r = await fetch(url, { signal });
  if (!r.ok)
    throw new Error(
      r.status === 404
        ? "This document is unavailable."
        : "The archive could not load. Please try again.",
    );
  return r.json();
}
export const originalDate = (doc: DocumentRecord) =>
  (doc.original_date === undefined ? doc.metadata.date : doc.original_date) ||
  "Date unknown";
const favoriteDocument = async (
  id: string,
  signal: AbortSignal,
  scope: string,
) => {
  const doc = await json<DocumentRecord>(
    `/api/documents/${encodeURIComponent(id)}`,
    signal,
  );
  if (doc.collection !== scope)
    throw new Error("Favorite is unavailable in this collection");
  return {
    title: doc.metadata.title,
    href: `/documents/${encodeURIComponent(id)}?collection=${encodeURIComponent(doc.collection)}`,
  };
};
export default function App() {
  const params = new URLSearchParams(location.search);
  const id = location.pathname.startsWith("/documents/")
    ? location.pathname.split("/")[2]
    : "";
  const [collections, setCollections] = useState<Collection[]>([]);
  const [listing, setListing] = useState<Listing | null>(null);
  const [doc, setDoc] = useState<DocumentRecord | null>(null);
  const [subjects, setSubjects] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [focus, setFocus] = useState(false);
  const [collectionID, setCollectionID] = useState(
    params.get("collection") || "",
  );
  const favorites = useBrowserFavorites(collectionID);
  useEffect(() => {
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setFocus(false);
    };
    window.addEventListener("keydown", escape);
    return () => window.removeEventListener("keydown", escape);
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    const timeout = setTimeout(() => controller.abort(), 15000);
    async function load() {
      const all = await json<Collection[]>(
        "/api/collections",
        controller.signal,
      );
      if (!active) return;
      setCollections(all);
      if (id) {
        const d = await json<DocumentRecord>(
          `/api/documents/${encodeURIComponent(id)}`,
          controller.signal,
        );
        if (active) {
          setDoc(d);
          setCollectionID(d.collection);
        }
      } else if (all.length) {
        const selected =
          new URLSearchParams(location.search).get("collection") || all[0].id;
        setCollectionID(selected);
        const q = new URLSearchParams(location.search);
        q.set("collection", selected);
        const [data, facets] = await Promise.all([
          json<Listing>(`/api/documents?${q}`, controller.signal),
          json<{ subjects: string[] }>(
            `/api/facets?collection=${encodeURIComponent(selected)}`,
            controller.signal,
          ),
        ]);
        if (active) {
          setListing(data);
          setSubjects(facets.subjects);
        }
      }
    }
    load()
      .catch((e) => {
        if (active)
          setError(
            controller.signal.aborted
              ? "The archive took too long to respond. Please try again."
              : e.message,
          );
      })
      .finally(() => {
        clearTimeout(timeout);
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      controller.abort();
      clearTimeout(timeout);
    };
  }, [id]);
  const collection = collections.find((c) => c.id === collectionID);
  const variant =
    doc?.variants.find((v) => v.id === params.get("variant")) ||
    doc?.variants[0];
  useEffect(() => {
    document.title = `${doc?.metadata.title || collection?.name || "Riot Grrrl zines"} · Riot Grrrl Archive`;
    let canonical = document.querySelector<HTMLLinkElement>(
      'link[rel="canonical"]',
    );
    if (!canonical) {
      canonical = document.createElement("link");
      canonical.rel = "canonical";
      document.head.append(canonical);
    }
    canonical.href = new URL(
      id
        ? `/documents/${id}`
        : collectionID
          ? `/?collection=${encodeURIComponent(collectionID)}`
          : "/",
      publicURL,
    ).href;
  }, [doc, collection, collectionID, id]);
  const offsetLink = (n: number) => {
    const q = new URLSearchParams(location.search);
    q.set("collection", collectionID);
    q.set("offset", String(n));
    return `/?${q}`;
  };
  return (
    <main
      className={focus ? "reading-focus" : ""}
      style={
        { "--accent": collection?.accent || "#9b1459" } as React.CSSProperties
      }
    >
      <header className="masthead">
        <a href="/" aria-label="Riot Grrrl Archive home">
          RIOT GRRRL
        </a>
        <span>Zines. Noise. Herstory.</span>
      </header>
      {loading && (
        <p role="status" className="notice">
          Loading the archive…
        </p>
      )}
      {error && (
        <div role="alert" className="notice">
          <p>{error}</p>
          <a href={location.href}>Try again</a>
        </div>
      )}
      {!loading && !error && !collections.length && (
        <section className="introduction">
          <p className="eyebrow">Collection in preparation</p>
          <h1>
            The Riot Grrrl
            <br />
            Zine Archive
          </h1>
          <p>No documents have been imported yet.</p>
        </section>
      )}
      {!loading && !error && collections.length > 0 && (
        <>
          <nav className="collection-nav" aria-label="Collections">
            {collections.map((c) => (
              <a
                key={c.id}
                href={`/?collection=${encodeURIComponent(c.id)}`}
                aria-current={c.id === collectionID ? "page" : undefined}
              >
                {c.name}
              </a>
            ))}
          </nav>
          {doc ? (
            <section className="document">
              <a href={`/?collection=${encodeURIComponent(doc.collection)}`}>
                ← Back to collection
              </a>
              <p className="eyebrow">{collection?.name}</p>
              <h1>{doc.metadata.title}</h1>
              <FavoriteButton
                id={doc.id}
                title={doc.metadata.title}
                favorites={favorites}
              />
              <ShareControls url={() => location.href} title={doc.metadata.title} />
              {params.get("read") === "1" && (
                <button aria-pressed={focus} onClick={() => setFocus(!focus)}>
                  {focus ? "Exit reading focus" : "Reading focus"}
                </button>
              )}
              <p className="description">
                {doc.metadata.creator || "Creator unknown"} ·{" "}
                {originalDate(doc)}
              </p>
              <p>{doc.metadata.subjects?.join(" · ")}</p>
              <p>Language: {doc.metadata.language || "Unknown"}</p>
              <aside className="access-policy" aria-label="Access and rights">
                <p>
                  <strong>
                    Rights: {doc.rights === "unknown" ? "Unknown" : doc.rights}.
                  </strong>{" "}
                  {doc.variants.some((v) => v.public_approved)
                    ? "Public reading and downloads are approved for archival access."
                    : "This record is available in the private archive; public access has not been approved."}{" "}
                  No reuse license is asserted.
                </p>
                <p>
                  Source:{" "}
                  <a
                    href={`https://archive.org/details/${encodeURIComponent(doc.source_item)}`}
                    rel="noreferrer"
                    target="_blank"
                  >
                    Internet Archive · {doc.source_item}
                  </a>
                  .{" "}
                  <a
                    href={`mailto:${contact}?subject=${encodeURIComponent(`Riot Grrrl Archive correction or removal: ${doc.metadata.title}`)}&body=${encodeURIComponent(`Document: ${new URL(`/documents/${doc.id}`, publicURL).href}\n\nRequest and supporting information:\n`)}`}
                  >
                    Request a correction or removal
                  </a>
                  .
                </p>
              </aside>
              <div className="variants">
                {doc.variants.map((v) => (
                  <article key={v.id}>
                    <h2>{v.filename}</h2>
                    <p>
                      {v.pages || "Unknown"} pages ·{" "}
                      {(v.size / 1048576).toFixed(1)} MB ·{" "}
                      {v.text_status === "page-text"
                        ? "Page text searchable"
                        : v.text_status === "document-text"
                          ? "Document text searchable; page mapping unavailable"
                          : "Text search unavailable"}
                    </p>
                    <a
                      className="button"
                      href={`?read=1&variant=${v.id}&page=1`}
                    >
                      Read on site
                    </a>{" "}
                    <a href={`/api/assets/${v.id}?download=1`}>
                      Download original PDF
                    </a>
                  </article>
                ))}
              </div>
              {params.get("read") === "1" && variant && (
                <Suspense fallback={<p role="status">Loading reader…</p>}>
                  <Reader
                    variant={variant}
                    initialPage={Number(params.get("page") || 1)}
                  />
                </Suspense>
              )}
              <details className="provenance">
                <summary>Source and preservation details</summary>
                <p>
                  Source:{" "}
                  <a
                    href={`https://archive.org/details/${encodeURIComponent(doc.source_item)}`}
                    target="_blank"
                    rel="noreferrer"
                  >
                    Internet Archive · {doc.source_item}
                  </a>
                </p>
                <p>Rights/access statement: {doc.rights}</p>
                <p>Files are served from preserved local copies.</p>
                {doc.variants.map((v) => (
                  <p className="hash" key={v.id}>
                    SHA-256: {v.sha256}
                  </p>
                ))}
              </details>
            </section>
          ) : (
            <Catalog
              collection={collection}
              collectionID={collectionID}
              listing={listing}
              subjects={subjects}
              params={params}
              offsetLink={offsetLink}
              favorites={favorites}
            />
          )}
        </>
      )}
      {collectionID && (
        <FavoriteShelf
          favorites={favorites}
          scope={collectionID}
          resolve={favoriteDocument}
        />
      )}
      <section className="about" id="about" aria-labelledby="about-title">
        <h2 id="about-title">About this archive</h2>
        <p>
          The Riot Grrrl Archive preserves feminist punk zines and serves
          original PDFs in a local reader. Catalog metadata and available
          document text are searchable. Source links identify the Internet
          Archive item each file came from.
        </p>
        <p>
          Public access is reviewed for each PDF variant. Availability here does
          not grant permission to reuse a document. Rights remain unknown unless
          stated, and This archive asserts no reuse license. Newly acquired
          editions remain private until reviewed.
        </p>
        <p>
          For corrections or removal requests, contact{" "}
          <a href={`mailto:${contact}`}>{contact}</a> with the document link and
          details of your request. Access can be withdrawn while preservation
          and decision records remain in the private archive.
        </p>
      </section>
      <footer>
        <span>Riot Grrrl Archive · Preserved with Arkhiv</span>
        <a href="#about">About, rights and contact</a>
      </footer>
    </main>
  );
}
