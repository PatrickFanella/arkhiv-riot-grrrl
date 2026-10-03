import { useState } from "react";
import type { Collection, DocumentRecord, Listing } from "./App";

function Cover({
  document: doc,
  eager = false,
}: {
  document: DocumentRecord;
  eager?: boolean;
}) {
  const [failed, setFailed] = useState(false);
  const variant = doc.variants[0];
  return (
    <a
      className="publication-cover"
      href={`/documents/${doc.id}`}
      aria-label={`Open ${doc.metadata.title}`}
    >
      {variant && !failed ? (
        <img
          src={`/api/assets/${variant.id}/cover`}
          alt=""
          loading={eager ? "eager" : "lazy"}
          onError={() => setFailed(true)}
        />
      ) : (
        <div className="cover-fallback">
          <span>Cover unavailable</span>
          <strong>{doc.metadata.title}</strong>
          <span>Open document →</span>
        </div>
      )}
    </a>
  );
}

export default function Catalog({
  collection,
  collectionID,
  listing,
  subjects,
  params,
  offsetLink,
}: {
  collection?: Collection;
  collectionID: string;
  listing: Listing | null;
  subjects: string[];
  params: URLSearchParams;
  offsetLink: (offset: number) => string;
}) {
  const riot = true;
  const unfiltered =
    !params.get("q") && !params.get("subject") && !listing?.offset;
  const featured = unfiltered ? listing?.documents[0] : undefined;
  const branded =
    collection?.name ===
    (riot ? "Riot Grrrl Zine Archive" : "The Anarchist Zine Archive");
  return (
    <section className="catalog" id="catalog">
      <div className="front-heading">
        <p className="eyebrow">
          {collectionID.startsWith("fixture-")
            ? "Synthetic verification collection"
            : "Independent print / preserved collection"}
        </p>
        <h1>
          {branded ? (
            <>
              {riot ? (
                <span className="cutletters" aria-label="Riot Grrrl">
                  <span aria-hidden="true">
                    {"RIOT GRRRL".split("").map((letter, index) =>
                      letter === " " ? (
                        <i className="cutspace" key={index} />
                      ) : (
                        <b className="cutletter" key={index}>
                          {letter}
                        </b>
                      ),
                    )}
                  </span>
                </span>
              ) : (
                "ANARCHIST"
              )}
              <span className="archive-subtitle">Zine Archive</span>
            </>
          ) : (
            collection?.name || "Collection unavailable"
          )}
        </h1>
        <p className="collection-introduction">{collection?.introduction}</p>
      </div>
      {featured && (
        <section className="front-feature" aria-label="From the collection">
          <div className="feature-copy">
            <p className="eyebrow">From the collection</p>
            <h2>
              <a href={`/documents/${featured.id}`}>
                {featured.metadata.title}
              </a>
            </h2>
            <p>{featured.metadata.creator || "Creator unknown"}</p>
            <p>{featured.metadata.date || "Date unknown"}</p>
            <a className="feature-action" href={`/documents/${featured.id}`}>
              Open document ↗
            </a>
            <p className="feature-foot">
              Original PDFs · Read on site · Download
            </p>
          </div>
          <div className="feature-book">
            <Cover document={featured} eager />
          </div>
          {(listing?.documents.length || 0) > 1 && (
            <aside
              className="feature-contents"
              aria-label="More from the collection"
            >
              <p className="eyebrow">Also inside</p>
              {listing?.documents.slice(1, 4).map((doc) => (
                <a key={doc.id} href={`/documents/${doc.id}`}>
                  <strong>{doc.metadata.title}</strong>
                  <small>{doc.metadata.creator || "Creator unknown"}</small>
                </a>
              ))}
            </aside>
          )}
        </section>
      )}
      <div className="catalog-heading">
        <h2>
          {params.get("q") || params.get("subject")
            ? "Search results"
            : "The collection"}
        </h2>
        <p role="status">
          {listing?.total || 0} documents
          {params.get("q") ? ` matching “${params.get("q")}”` : ""}
        </p>
      </div>
      <form method="get" action="/" className="search">
        <input type="hidden" name="collection" value={collectionID} />
        <label>
          Search titles and available text
          <input
            name="q"
            type="search"
            defaultValue={params.get("q") || ""}
            maxLength={200}
          />
        </label>
        <label>
          Subject
          <select name="subject" defaultValue={params.get("subject") || ""}>
            <option value="">All subjects</option>
            {subjects.map((subject) => (
              <option key={subject}>{subject}</option>
            ))}
          </select>
        </label>
        <button type="submit">Search</button>
      </form>
      {listing?.documents.length === 0 && (
        <p className="notice">
          No documents match these filters.{" "}
          <a href={`/?collection=${encodeURIComponent(collectionID)}`}>
            Clear filters
          </a>
        </p>
      )}
      <div className="publication-grid">
        {listing?.documents.map((doc) => (
          <article key={doc.id} className="publication">
            <Cover document={doc} />
            <div className="publication-caption">
              <p className="publication-subjects">
                {doc.metadata.subjects?.join(" / ")}
              </p>
              <h2>
                <a href={`/documents/${doc.id}`}>{doc.metadata.title}</a>
              </h2>
              <p>
                {doc.metadata.creator || "Creator unknown"} ·{" "}
                {doc.metadata.date || "Date unknown"}
              </p>
              <p className="rights-label">
                Rights: {doc.rights === "unknown" ? "Unknown" : doc.rights} · No
                reuse license asserted
              </p>
              {doc.hits.map((hit, index) => (
                <p className="search-hit" key={index}>
                  <a
                    href={`/documents/${doc.id}?read=1&variant=${hit.variant}${hit.page > 0 ? `&page=${hit.page}` : ""}`}
                  >
                    {hit.page > 0 ? `Page ${hit.page}` : "Document text"}
                  </a>
                  : {hit.snippet}
                </p>
              ))}
            </div>
          </article>
        ))}
      </div>
      {listing && (
        <nav className="pagination" aria-label="Catalog pages">
          {listing.offset > 0 && (
            <a href={offsetLink(Math.max(0, listing.offset - listing.limit))}>
              Previous
            </a>
          )}
          {listing.offset + listing.limit < listing.total && (
            <a href={offsetLink(listing.offset + listing.limit)}>Next</a>
          )}
        </nav>
      )}
    </section>
  );
}
