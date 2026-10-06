// Product-owned controls. Runtime catalog access remains with this product.
import { useEffect, useRef, useState } from "react";

const memory = new Map<string, string[]>();
const failedWrites = new Set<string>();
const temporaryFavoritesNotice =
  "Browser storage is unavailable. These favorites last while this page remains open.";
export function useBrowserFavorites(scope: string) {
  const key = `archive.product-favorites.v1:${scope}`;
  const read = () => {
    if (failedWrites.has(key)) return memory.get(key) || [];
    try {
      const value: unknown = JSON.parse(localStorage.getItem(key) || "[]");
      if (Array.isArray(value))
        return [
          ...new Set(
            value.filter(
              (id): id is string =>
                typeof id === "string" &&
                /^[A-Za-z0-9][A-Za-z0-9:._-]{0,299}$/.test(id),
            ),
          ),
        ].slice(0, 5000);
    } catch {
      /* Keep this page usable when storage fails. */
    }
    return memory.get(key) || [];
  };
  const [state, setState] = useState(() => ({ key, ids: read() })),
    [notice, setNotice] = useState(() =>
      failedWrites.has(key) ? temporaryFavoritesNotice : "",
    );
  useEffect(() => {
    setState({ key, ids: read() });
    setNotice(failedWrites.has(key) ? temporaryFavoritesNotice : "");
    const changed = (event: StorageEvent) => {
      if (event.key === key) setState({ key, ids: read() });
    };
    window.addEventListener("storage", changed);
    return () => window.removeEventListener("storage", changed);
  }, [key]);
  const current = useRef(state);
  current.current = state;
  const toggle = (id: string) => {
    const value = current.current;
    if (value.key !== key || !/^[A-Za-z0-9][A-Za-z0-9:._-]{0,299}$/.test(id))
      return;
    const ids = value.ids.includes(id)
      ? value.ids.filter((x) => x !== id)
      : [...value.ids, id];
    if (ids.length > 5000) {
      setNotice(
        "The browser favorite limit is 5,000. Remove a favorite before adding another.",
      );
      return;
    }
    memory.set(key, ids);
    try {
      localStorage.setItem(key, JSON.stringify(ids));
      failedWrites.delete(key);
      setNotice("");
    } catch {
      failedWrites.add(key);
      setNotice(temporaryFavoritesNotice);
    }
    const next = { key, ids };
    current.current = next;
    setState(next);
  };
  return { ids: state.key === key ? state.ids : [], toggle, notice };
}
export type BrowserFavorites = ReturnType<typeof useBrowserFavorites>;
export function FavoriteButton({
  id,
  title,
  favorites,
}: {
  id: string;
  title: string;
  favorites: BrowserFavorites;
}) {
  const saved = favorites.ids.includes(id);
  return (
    <button
      className="key favorite-button"
      aria-pressed={saved}
      aria-label={`${saved ? "Remove" : "Save"} favorite ${title}`}
      onClick={() => favorites.toggle(id)}
    >
      {saved ? "Saved favorite" : "Save favorite"}
    </button>
  );
}
export type FavoriteRecord = { title: string; href: string };
export function FavoriteShelf({
  favorites,
  scope,
  resolve,
}: {
  favorites: BrowserFavorites;
  scope: string;
  resolve: (
    id: string,
    signal: AbortSignal,
    scope: string,
  ) => Promise<FavoriteRecord>;
}) {
  const [page, setPage] = useState(1),
    [records, setRecords] = useState<Record<string, FavoriteRecord>>({}),
    [loaded, setLoaded] = useState(false);
  const currentPage = Math.min(
      page,
      Math.max(1, Math.ceil(favorites.ids.length / 24)),
    ),
    ids = favorites.ids.slice((currentPage - 1) * 24, currentPage * 24),
    signature = JSON.stringify(ids);
  useEffect(() => {
    const controller = new AbortController();
    setLoaded(false);
    setRecords({});
    void Promise.all(
      (JSON.parse(signature) as string[]).map(async (id) => {
        try {
          return [id, await resolve(id, controller.signal, scope)] as const;
        } catch {
          return null;
        }
      }),
    ).then((values) => {
      if (!controller.signal.aborted) {
        setRecords(
          Object.fromEntries(values.filter((value) => value !== null)),
        );
        setLoaded(true);
      }
    });
    return () => controller.abort();
  }, [signature, scope, resolve]);
  function exportFavorites() {
    const url = URL.createObjectURL(
      new Blob(
        [
          JSON.stringify(
            { version: 1, collection: scope, favorites: favorites.ids },
            null,
            2,
          ),
        ],
        { type: "application/json" },
      ),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "archive-browser-favorites.json";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  return (
    <section className="viewer-preferences" aria-label="Browser favorites">
      <h2>Saved favorites</h2>
      <p>
        {favorites.ids.length} references saved in this browser for {scope}.
      </p>
      {favorites.notice && <p role="status">{favorites.notice}</p>}
      <button className="key" onClick={exportFavorites}>
        Export browser favorites
      </button>
      <ul>
        {ids.map((id) => (
          <li key={id}>
            {records[id] ? (
              <a href={records[id].href}>{records[id].title}</a>
            ) : (
              <span>
                {loaded
                  ? "Unavailable reference retained"
                  : "Checking current availability"}
                : {id}
              </span>
            )}{" "}
            <button className="key" onClick={() => favorites.toggle(id)}>
              Remove favorite
            </button>
          </li>
        ))}
      </ul>
      {!ids.length && <p>No saved favorites.</p>}
      {favorites.ids.length > 24 && (
        <nav aria-label="Favorite pages">
          <button
            className="key"
            disabled={currentPage === 1}
            onClick={() => setPage(currentPage - 1)}
          >
            Previous favorites
          </button>
          <span>Page {currentPage}</span>
          <button
            className="key"
            disabled={currentPage * 24 >= favorites.ids.length}
            onClick={() => setPage(currentPage + 1)}
          >
            Next favorites
          </button>
        </nav>
      )}
    </section>
  );
}
export function ShareControls({ url, title }: { url: string | (() => string); title: string }) {
  const [status, setStatus] = useState(""),
    [fallback, setFallback] = useState("");
  const currentURL = () => typeof url === "function" ? url() : url;
  const socialURL = () => `https://bsky.app/intent/compose?text=${encodeURIComponent(`${title} ${currentURL()}`)}`;
  const emailURL = () => `mailto:?subject=${encodeURIComponent(title)}&body=${encodeURIComponent(currentURL())}`;
  async function share() {
    const link = currentURL();
    try {
      if (navigator.share) await navigator.share({ url: link, title });
      else {
        await navigator.clipboard.writeText(link);
        setStatus("Link copied.");
      }
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      setFallback(link);
      setStatus("Select and copy this link.");
    }
  }
  return (
    <div className="share-controls">
      <button
        className="key"
        onClick={() => {
          void share();
        }}
      >
        Share item
      </button>
      <a
        href={socialURL()}
        onClick={event => {event.currentTarget.href = socialURL();}}
        target="_blank"
        rel="noreferrer"
      >
        Share on Bluesky
      </a>
      <a
        href={emailURL()}
        onClick={event => {event.currentTarget.href = emailURL();}}
      >
        Share by email
      </a>
      {status && <p role="status">{status}</p>}
      {fallback && (
        <label>
          Share link
          <input
            readOnly
            value={fallback}
            onFocus={(event) => event.currentTarget.select()}
          />
        </label>
      )}
    </div>
  );
}
