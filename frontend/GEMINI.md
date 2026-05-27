# Meltica Control Client

## Project Overview

This is a Next.js web client for managing Meltica trading gateway strategies and configurations. It provides a user interface for interacting with the Meltica trading gateway API, allowing users to manage strategies, providers, adapters, risk limits, and more.

**Key Technologies:**

*   **Framework:** Next.js 16 with App Router
*   **Language:** TypeScript
*   **Styling:** Tailwind CSS v4
*   **Components:** shadcn/ui
*   **Data Layer:** TanStack React Query v5
*   **Package Manager:** pnpm
*   **Testing:** Vitest, Playwright, MSW

## Building and Running

### Prerequisites

*   Node.js 22.20.0 or higher
*   pnpm 10.20.0 or higher
*   Meltica gateway running on `http://localhost:8880`

### Getting Started

1.  **Install dependencies:**
    ```bash
    pnpm install
    ```

2.  **Configure API endpoint** (optional):

    Create or edit `.env.local`:
    ```
    NEXT_PUBLIC_API_URL=http://localhost:8880
    ```

3.  **Start the development server:**
    ```bash
    pnpm dev
    ```

4.  **Open the application:**

    Navigate to [http://localhost:3000](http://localhost:3000)

### Available Scripts

*   `pnpm dev`: Start development server with Turbopack.
*   `pnpm build`: Build production bundle.
*   `pnpm start`: Start production server.
*   `pnpm lint`: Run ESLint.
*   `pnpm test`: Run unit/integration tests (Vitest + MSW).
*   `pnpm test:unit:watch`: Watch mode for the Vitest suite.
*   `pnpm test:e2e`: Run Playwright smoke tests.
*   `pnpm generate:api-types`: Generate TypeScript types from the OpenAPI specification.

## Development Conventions

### Data Layer

All API access flows through domain-specific modules in `src/lib/api/`. Each module exports request helpers plus Zod schemas to validate responses before they reach React Query caches. Corresponding hooks live under `src/lib/hooks/` and wrap common queries or mutations with cache keys, toast notifications, and error handling.

The core HTTP client is in `src/lib/api/http.ts` and provides a robust way to make API requests with features like request timeouts, telemetry headers, and Zod schema validation.

### Component Library

This project uses [shadcn/ui](https://ui.shadcn.com/) components. To add new components, run:

```bash
pnpm dlx shadcn-ui@latest add <component-name>
```

### Theming

Theming is handled using CSS variables and the `theme-provider.tsx` component. Global color tokens are in `src/app/globals.css`.

### Testing

*   **Unit & hook tests:** Vitest and MSW are used for unit and hook tests. MSW handlers are in `src/mocks/handlers.ts`.
*   **E2E tests:** Playwright is used for end-to-end smoke tests.
