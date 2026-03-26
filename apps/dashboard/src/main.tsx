import React from "react";
import ReactDOM from "react-dom/client";

import App from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import "./styles/index.css";

const faviconURL = `/PactMigrate-icon.png?v=${Date.now()}`;
let faviconEl = document.querySelector("link[rel='icon']") as HTMLLinkElement | null;
if (!faviconEl) {
  faviconEl = document.createElement("link");
  faviconEl.rel = "icon";
  document.head.appendChild(faviconEl);
}
faviconEl.type = "image/png";
faviconEl.href = faviconURL;

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>,
);

