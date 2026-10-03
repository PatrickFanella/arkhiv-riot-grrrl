import { lazy, Suspense, StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./styles.css";

const DesignPrototype = import.meta.env.DEV && new URLSearchParams(location.search).has("design")
  ? lazy(() => import("./DesignPrototype"))
  : null;

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    {DesignPrototype ? <Suspense fallback={<p>Loading design study…</p>}><DesignPrototype /></Suspense> : <App />}
  </StrictMode>,
);
