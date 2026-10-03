// Throwaway catalog studies. Choose a direction, then absorb the winner and delete this file.
import { useEffect, useRef, useState } from "react";
import "./design-prototype.css";

const riot = true;
const names = riot ? ["Cut & paste", "Copy shop", "Bedroom press"] : ["Front cover", "Editorial spread", "The index"];
const records = riot ? [
  { title: "Girl Germs", issue: "03", creator: "Allison Wolfe & Molly Neuman", date: "1991", tag: "Girl Germs" },
  { title: "VOOP", issue: "01", creator: "Greta Drummond-Gillett & Layla Gibbon", date: "1992–93", tag: "VOOP" },
  { title: "Drop Babies", issue: "01", creator: "Layla Gibbon, Greta Drummond-Gillett & Rosie", date: "1992", tag: "Drop Babies" },
  { title: "Girl Germs", issue: "04", creator: "Allison Wolfe & Molly Neuman", date: "1991", tag: "Girl Germs" },
  { title: "VOOP", issue: "03", creator: "Greta Drummond-Gillett & Layla Gibbon", date: "1992–93", tag: "VOOP" },
  { title: "VOOP", issue: "05", creator: "Greta Drummond-Gillett & Layla Gibbon", date: "1992–93", tag: "VOOP" },
] : [
  { title: "A Crime Called Freedom", issue: "01", creator: "Os Cangaceiros", date: "Date unknown", tag: "Theory" },
  { title: "2 by Le Guin", issue: "02", creator: "Ursula K. Le Guin", date: "Date unknown", tag: "Literature" },
  { title: "Against the Production Ethic", issue: "03", creator: "Creator not recorded", date: "Date unknown", tag: "Work" },
  { title: "An Anarchist Free Herbal", issue: "04", creator: "Second edition", date: "Date unknown", tag: "Mutual aid" },
  { title: "10 Preliminary Theses on Resistance", issue: "05", creator: "Creator not recorded", date: "Date unknown", tag: "Theory" },
  { title: "A Brief Introduction to Anarchism", issue: "06", creator: "Creator not recorded", date: "Date unknown", tag: "Theory" },
];
type RecordEntry = typeof records[number];

function Cover({ record, index = 0 }: { record: RecordEntry; index?: number }) {
  return <div className={`study-cover cover-${index % 6}`} aria-label={`Typeset study for ${record.title}; not the original cover`}>
    <span className="cover-top">{riot ? "An independent publication" : record.creator}</span>
    <strong>{record.title}</strong>
    <div className="cover-art" aria-hidden="true"><i /><i /><i /><span>{riot ? "✶" : "A"}</span></div>
    <span className="cover-bottom">{riot ? `Issue ${record.issue} / ${record.date}` : "WORDS / IDEAS / RESISTANCE"}</span>
  </div>;
}

export default function DesignPrototype() {
  const initial = new URLSearchParams(location.search).get("design") || "A";
  const [variant, setVariant] = useState(["A", "B", "C"].includes(initial) ? initial : "A");
  const [query, setQuery] = useState("");
  const [tag, setTag] = useState("All");
  const [selected, setSelected] = useState<RecordEntry | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => { if (selected) dialog.current?.showModal(); }, [selected]);
  const sourceURL = riot ? "https://archive.org/details/girl-germs-3" : "https://archive.org/details/ZineArchive";
  const selectedSourceURL = riot && selected ? `https://archive.org/details/${selected.title === "Girl Germs" ? `girl-germs-${Number(selected.issue)}` : selected.title === "VOOP" ? `voop-issue-${Number(selected.issue)}-uk-riot-grrl-zine` : "drop-babies-zine-issue-1-uk-riot-grrrl"}` : sourceURL;
  const index = ["A", "B", "C"].indexOf(variant);
  const filtered = records.filter(r => (tag === "All" || r.tag === tag) && `${r.title} ${r.creator}`.toLowerCase().includes(query.toLowerCase()));
  const switchTo = (next: string) => {
    setVariant(next);
    const url = new URL(location.href); url.searchParams.set("design", next); history.replaceState(null, "", url);
    window.scrollTo({ top: 0 });
  };
  useEffect(() => {
    const handle = (event: KeyboardEvent) => {
      if (event.target instanceof HTMLElement && (event.target.closest("input, textarea, select, button") || event.target.isContentEditable)) return;
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        const next = (index + (event.key === "ArrowRight" ? 1 : 2)) % 3;
        const key = ["A", "B", "C"][next]; setVariant(key);
        const url = new URL(location.href); url.searchParams.set("design", key); history.replaceState(null, "", url);
      }
      if (event.key === "Escape") setSelected(null);
    };
    document.addEventListener("keydown", handle);
    return () => document.removeEventListener("keydown", handle);
  }, [index]);
  useEffect(() => { document.title = `${riot ? "Riot Grrrl" : "Anarchist"} · ${names[index]} · Design study`; }, [index]);
  const book = (r: RecordEntry, i: number) => <button className="study-book" key={`${r.title}-${r.issue}`} onClick={() => setSelected(r)}>
    <Cover record={r} index={i} /><span className="book-caption"><span>{r.tag}</span><strong>{r.title}{riot ? ` #${r.issue}` : ""}</strong><small>{r.creator}</small></span>
  </button>;
  const title = riot ? "RIOT GRRRL" : "ANARCHIST";
  return <div className={`design-study ${riot ? "riot" : "politik"} variant-${variant.toLowerCase()}`}>
    <a className="study-skip" href="#study-catalog">Skip to catalog</a>
    <div className="study-disclaimer">DESIGN STUDY / {variant} — {names[index]} <span>Sample selections · original typeset artwork, not scanned covers</span></div>
    <header className="study-header"><a href={`/?design=${variant}`} className="study-mark">{riot ? "RG /" : "A /"}</a><nav aria-label="Study navigation"><a href="#study-catalog">The archive</a><a href="#study-about">About the collection</a><a href={sourceURL} target="_blank" rel="noreferrer">Source example ↗</a></nav><span className="header-note">Independent print.<br />Preserved.</span></header>
    {variant === "A" && <>
      <section className="front-heading"><p className="study-kicker">{riot ? "Made loud. Made by hand." : "The independent print archive"}</p><h1 aria-label={`${title} Zine Archive`}>{riot ? <span className="cutletters" aria-hidden="true">{title.split("").map((letter, i) => <b className={letter === " " ? "cutspace" : "cutletter"} key={i}>{letter}</b>)}</span> : title}<span>{riot ? "ZINE ARCHIVE" : "ZINE ARCHIVE /"}</span></h1></section>
      <section className="front-feature"><div className="feature-copy"><span className="study-kicker">{riot ? "PASS IT ON →" : "01 / FROM THE COLLECTION"}</span><h2>{riot ? <>A little paper.<br />A lot to say.</> : <>Ideas worth<br />keeping alive.</>}</h2><p>{riot ? "Girl Germs, VOOP and Drop Babies. Explore independent voices through the pages they made." : "A collection of independent writing on freedom, work, mutual aid, and the possibilities of another world."}</p><a className="study-action" href="#study-catalog">{riot ? "Dig through the zines ↗" : "Explore the archive ↗"}</a><span className="feature-foot">{riot ? "READ / COPY / REMEMBER" : "PAMPHLETS. ESSAYS. INDEPENDENT PRINT."}</span></div><button className="feature-book" onClick={() => setSelected(records[0])}><Cover record={records[0]} /><span className="feature-sticker">{riot ? "YES!" : "SELECTED\nREADING"}</span></button><aside className="feature-aside"><span className="study-kicker">{riot ? "ON THE TABLE" : "ALSO INSIDE"}</span>{records.slice(1, 4).map((r, i) => <button key={r.title} onClick={() => setSelected(r)}><span>0{i + 2}</span><strong>{r.title}</strong><small>{r.tag}</small></button>)}<div className="margin-note">{riot ? "Your voice belongs here. ✶" : "Read a page.\nFollow an idea."}</div></aside></section>
    </>}
    {variant === "B" && <>
      <section className="spread-heading"><span className="study-kicker">{riot ? "NO GLOSS. ALL VOICE." : "A READING ROOM FOR INDEPENDENT IDEAS"}</span><h1>{riot ? <>RIOT<br /><em>GRRRL!</em></> : <>The<br /><em>Anarchist</em><br />Zine Archive.</>}</h1><div className="spread-intro"><span className="spread-number">{riot ? "✶" : "01"}</span><p>{riot ? "Stapled. Photocopied. Passed from hand to hand. A home for the pages that made their own space." : "Independent print opens a space for ideas that refuse to stay in the margins. Start with a text. See where it takes you."}</p><a href="#study-catalog" className="study-action">{riot ? "GET INTO IT →" : "Browse the collection →"}</a></div></section>
      <section className="spread-feature"><button onClick={() => setSelected(records[0])}><Cover record={records[0]} /></button><div><span className="study-kicker">{riot ? "FROM THE PHOTOCOPIER" : "IN FOCUS / THEORY"}</span><h2>{records[0].title}</h2><p>{records[0].creator}</p><button className="text-link" onClick={() => setSelected(records[0])}>Explore this publication ↗</button></div><div className="spread-secondary"><Cover record={records[1]} index={1} /><span>{riot ? "MAKE SPACE. MAKE NOISE." : "A different voice on every page."}</span></div></section>
    </>}
    {variant === "C" && <section className="index-heading"><div><span className="study-kicker">{riot ? "THE BEDROOM PRESS / A PERSONAL LIBRARY" : "A CATALOG OF INDEPENDENT PRINT"}</span><h1>{riot ? <>dear<br /><em>riot grrrl,</em></> : <>The<br />Index<span>↘</span></>}</h1></div><div className="index-note"><p>{riot ? "These are the pages we kept. The ones folded into bags, swapped with friends, and read until the staples gave out." : "Browse by title, follow a subject, or find a familiar author. Every entry is a doorway into the collection."}</p><span>{riot ? "love, the archive ♡" : "ARCHIVE / READING / PRESERVATION"}</span></div></section>}
    <section id="study-catalog" className="study-catalog"><div className="catalog-heading"><h2>{riot ? (variant === "C" ? "the zine shelf" : "The zine pile") : (variant === "C" ? "Titles A—Z" : "From the shelves")}</h2><span>{filtered.length} sample selections / {riot ? "1991–1993" : "Independent print"}</span></div><div className="study-filters"><div className="study-tabs" aria-label={riot ? "Filter by publication" : "Filter by subject"}>{["All", ...new Set(records.map(r => r.tag))].map(t => <button key={t} aria-pressed={tag === t} onClick={() => setTag(t)}>{t}</button>)}</div><label className="study-search"><span className="sr-only">Search sample titles and creators</span><input value={query} onChange={e => setQuery(e.target.value)} placeholder="Find a title or a voice…" /><span aria-hidden="true">↗</span></label></div>
      {filtered.length === 0 && <p role="status">No sample titles match. Try another search or choose All.</p>}
      {variant === "C" && !riot ? <div className="index-list">{filtered.map((r, i) => <button key={r.title} onClick={() => setSelected(r)}><span className="index-order">{String(i + 1).padStart(2, "0")}</span><strong>{r.title}</strong><span>{r.creator}</span><span className="index-tag">{r.tag}</span><span>↗</span></button>)}</div> : <div className="study-books">{filtered.map(book)}</div>}
    </section>
    <footer id="study-about" className="study-footer"><strong>{riot ? "Made by hand.\nKept with care." : "Independent print.\nA lasting place."}</strong><p>Browse, read, and discover independent publications.<br />This preview compares layouts; the current catalog contains source details and access information.</p><a href={sourceURL} target="_blank" rel="noreferrer">Explore the source ↗</a></footer>
    {selected && <dialog ref={dialog} onClose={() => setSelected(null)} className="study-detail" aria-label={`${selected.title} design preview`}><button autoFocus className="detail-close" onClick={() => dialog.current?.close()}>Close ×</button><Cover record={selected} index={records.indexOf(selected)} /><div><p className="study-kicker">PUBLICATION / {selected.tag}</p><h2>{selected.title}{riot ? ` #${selected.issue}` : ""}</h2><p>{selected.creator}</p><p>{selected.date}</p><p>This is a layout study with original typeset cover art. Use the source collection link to explore the original publications. The qualified reader is separate from this design study.</p><a className="study-action" href={selectedSourceURL} target="_blank" rel="noreferrer">View source record ↗</a></div></dialog>}
    <div className="study-switcher" aria-label="Design preview selector"><button aria-label="Previous design" onClick={() => switchTo(["A", "B", "C"][(index + 2) % 3])}>←</button><span aria-live="polite">{variant} / {names[index]}</span>{["A", "B", "C"].map(k => <button key={k} aria-label={`Design ${k}`} aria-pressed={variant === k} onClick={() => switchTo(k)}>{k}</button>)}<button aria-label="Next design" onClick={() => switchTo(["A", "B", "C"][(index + 1) % 3])}>→</button></div>
  </div>;
}
