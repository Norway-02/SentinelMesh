import '@testing-library/jest-dom';

// Mock ResizeObserver for recharts in jsdom
class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(global as any).ResizeObserver = MockResizeObserver;

// Mock EventSource globally for testing environment
class MockEventSource {
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  url: string;

  constructor(url: string) {
    this.url = url;
    setTimeout(() => {
      if (this.onopen) this.onopen();
    }, 10);
  }

  close() {}
}

(global as any).EventSource = MockEventSource;
