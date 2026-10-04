/**
 * Test setup: registers the jest-dom matchers (toHaveValue, toBeDisabled, …)
 * with Vitest's expect, and unmounts trees between tests.
 */
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

afterEach(() => {
  cleanup();
  // Node's localStorage is only available behind a flag on this runtime, and
  // these tests don't depend on it — clear it only when it actually exists.
  globalThis.localStorage?.clear();
});