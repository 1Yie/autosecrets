import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button } from "./ui/button";

interface ErrorBoundaryProps {
  children?: ReactNode;
  fallback?: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

/** Catches render errors below it so one failed section never crashes the
 * whole application (frontend-guidelines: route and feature boundaries). */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("ErrorBoundary caught:", error, info.componentStack);
  }

  render(): ReactNode {
    if (this.state.error) {
      return (
        this.props.fallback ?? (
          <div className="rounded border border-red-500/40 bg-red-500/10 p-4 text-sm">
            <p className="font-semibold text-red-500">页面出错了</p>
            <p className="mt-1 opacity-80">{this.state.error.message}</p>
            <Button variant="outline" className="mt-3"
              
              onClick={() => this.setState({ error: null })}
            >
              重试
            </Button>
          </div>
        )
      );
    }
    return this.props.children ?? null;
  }
}
