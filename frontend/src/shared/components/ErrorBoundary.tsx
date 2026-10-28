import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  /** Rendered instead of the children after one of them throws while rendering. */
  fallback: (error: unknown, reset: () => void) => ReactNode
  children: ReactNode
}

interface State {
  error: unknown
  failed: boolean
}

/**
 * Stops a render error in one part of the tree from blanking the whole app.
 * Give it a `key` that changes with the route, so navigating away clears the error.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null, failed: false }

  static getDerivedStateFromError(error: unknown): State {
    return { error, failed: true }
  }

  componentDidCatch(error: unknown, info: ErrorInfo): void {
    console.error('Render error', error, info.componentStack)
  }

  reset = (): void => this.setState({ error: null, failed: false })

  render(): ReactNode {
    return this.state.failed ? this.props.fallback(this.state.error, this.reset) : this.props.children
  }
}
