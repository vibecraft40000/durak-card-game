import { Component, type ErrorInfo, type ReactNode } from "react";
import { AppButton } from "@/shared/ui/Button";

type Props = {
  children: ReactNode;
  fallback?: ReactNode;
};

type State = {
  hasError: boolean;
  error: Error | null;
};

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("[ErrorBoundary]", error, errorInfo);
  }

  render() {
    if (this.state.hasError && this.state.error) {
      if (this.props.fallback) {
        return this.props.fallback;
      }
      return (
        <div className="screen" style={{ padding: 20, textAlign: "center" }}>
          <h2 className="screen__title">Что-то пошло не так</h2>
          <p className="card__hint" style={{ marginBottom: 16 }}>
            {this.state.error.message}
          </p>
          <AppButton
            type="button"
            variant="primary"
            onClick={() => this.setState({ hasError: false, error: null })}
          >
            Попробовать снова
          </AppButton>
        </div>
      );
    }
    return this.props.children;
  }
}
