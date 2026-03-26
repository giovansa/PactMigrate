import React from "react";

type Props = {
  children: React.ReactNode;
};

type State = {
  hasError: boolean;
  message: string;
};

export class ErrorBoundary extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, message: "" };
  }

  static getDerivedStateFromError(error: unknown): State {
    const message = error instanceof Error ? error.message : String(error);
    return { hasError: true, message };
  }

  componentDidCatch(error: unknown) {
    // Keep a trace in console for debugging while showing friendly UI.
    // eslint-disable-next-line no-console
    console.error("Dashboard runtime error:", error);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{ padding: 24, fontFamily: "ui-sans-serif, system-ui, sans-serif" }}>
          <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 8 }}>
            Dashboard failed to render
          </h1>
          <p style={{ marginBottom: 8 }}>
            Check this error and refresh the page after fixing it:
          </p>
          <pre style={{ whiteSpace: "pre-wrap", background: "#111827", color: "#f9fafb", padding: 12, borderRadius: 8 }}>
            {this.state.message || "Unknown runtime error"}
          </pre>
        </div>
      );
    }
    return this.props.children;
  }
}

