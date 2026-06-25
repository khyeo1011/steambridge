import '@testing-library/jest-dom/vitest'

// Required for React 18's act() integration in test environments.
;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true
