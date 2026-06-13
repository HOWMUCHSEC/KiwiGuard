import React from "react";
import ReactDOM from "react-dom/client";

import "@carbon/styles/css/styles.css";
import { App } from "app/App";
import { AppProviders } from "app/providers/AppProviders";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <AppProviders>
      <App />
    </AppProviders>
  </React.StrictMode>
);
