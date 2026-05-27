### Codebase Analysis Report: Meltica Control Plane

---

#### 1. Application Overview & Purpose

The repository contains the source code for **Meltica Control**, a web-based client application that serves as a user interface for the "Meltica Control Plane."

Based on the API specification (`frontend-api.yaml`) and frontend code, this application is a sophisticated dashboard for managing and monitoring **automated or algorithmic trading strategies**. It allows users to define trading logic in JavaScript, deploy it, connect it to data/execution providers (like cryptocurrency exchanges), and manage its lifecycle and risk parameters.

---

#### 2. Core Functionality

The application is built around several key concepts:

*   **Strategies:** Abstract definitions of a trading strategy, including its name, description, and configuration schema.
*   **Strategy Modules:** The concrete implementation of a strategy, written in JavaScript. Users can upload, version (via content hash and tags like `latest`), and manage these code modules. The UI includes a code editor (`react-ace`) for this purpose.
*   **Instances:** A running, configured instance of a `Strategy Module`. This is the live entity that executes trades. Users can start, stop, and monitor instances.
*   **Providers & Adapters:** `Providers` are connections to external services (e.g., exchanges, data feeds). `Adapters` are the "types" of providers, defining their capabilities and required settings (e.g., API keys).
*   **Risk Management:** A dedicated section for setting global risk limits, such as maximum position size, order frequency, and a "kill switch" to halt all activity.
*   **Context Management:** A feature to back up and restore the entire application's configuration (providers, instances, risk settings) from a single file.

---

#### 3. Navigation & User Workflows

The application features a clear, resource-oriented navigation structure. The main user flows are:

*   **Navigation:** A persistent top navigation bar provides links to each major section:
    *   `Dashboard`
    *   `Instances`
    *   `Strategies` & `Strategy Modules`
    *   `Providers` & `Adapters`
    *   `Risk Limits`
    *   `Context Backup`

*   **Primary User Workflow (Strategy Deployment):**
    1.  A user navigates to **Strategies -> Strategy Modules** to upload a JavaScript file containing their trading logic.
    2.  They then go to the **Instances** page to create a new instance.
    3.  During creation, they select the `Strategy Module` to use, configure its parameters, and assign `Providers` to it (e.g., use "Binance" for market data and "Coinbase" for execution).
    4.  Once created, the user can **start** the instance from the Instances dashboard.
    5.  They can monitor its performance by viewing its recent orders and executions.

*   **Configuration Workflow:**
    1.  A user visits the **Providers** page to configure connections to exchanges, providing API keys and other settings as defined by the corresponding **Adapter**.
    2.  They visit the **Risk** page to set global safety limits that apply to all running strategies.

---

#### 4. Technology Stack & Architectural Patterns

The application is built with a modern and robust technology stack, adhering to excellent design patterns.

*   **Framework:** **Next.js 16** (App Router) with **React 19** and **TypeScript**.
*   **UI:**
    *   A custom component library built with **Tailwind CSS** and **Radix UI** primitives (likely following **shadcn/ui** methodology).
    *   Icons are provided by **Lucide React**.
    *   The UI pattern is consistent: pages typically display a list of resources (e.g., strategies), and clicking an item opens a slide-out `Sheet` for detailed viewing and management.
*   **State Management:**
    *   **Server State:** **TanStack Query (React Query)** is used extensively for all API interactions. This handles data fetching, caching, and synchronization, with clear loading and error states.
    *   **Client State:** Global client-side state (like theme) is managed via React Context in `ClientProviders`.
*   **API Communication:**
    *   The frontend is strictly typed against the backend API. The `openapi-typescript` tool generates TypeScript types from the `frontend-api.yaml` OpenAPI specification, ensuring the client and server are always in sync.
    *   A well-structured set of custom hooks (e.g., `useStrategiesQuery`, `useCreateInstanceMutation`) abstracts away the API fetching logic and provides a simple, declarative interface for components.
*   **Testing:**
    *   **Unit/Component Tests:** **Vitest** and **React Testing Library**.
    *   **End-to-End (E2E) Tests:** **Playwright**.
    *   **API Mocking:** **Mock Service Worker (MSW)** is used to mock API responses during testing, allowing for isolated and reliable frontend tests.

### Proposed Upgrades

---

The codebase is clean, modern, and well-architected. The following proposals aim to build on this strong foundation.

#### 1. Implement a Component Storybook

The project has an excellent component library in `src/components/ui`.

*   **Proposal:** Integrate **Storybook**. This would create an isolated environment for developing, viewing, and testing each UI component.
*   **Benefit:** It would improve developer productivity, enforce UI consistency, and serve as living documentation for the design system, making it easier to onboard new developers and build features faster.

#### 2. Enhance Real-time Capabilities with WebSockets

For a trading application, real-time data is crucial. The current architecture relies on polling via TanStack Query's `staleTime`.

*   **Proposal:** Augment the existing API with a WebSocket connection to push real-time updates to the client for:
    *   Instance status changes (e.g., `running` -> `stopped`).
    *   New orders and executions.
    *   Provider balance updates.
*   **Benefit:** This would make the UI significantly more responsive and provide users with immediate feedback, which is critical for a control plane managing live operations.

#### 3. Build Out the Main Dashboard

The `Dashboard` page is currently a placeholder. It could be transformed into a valuable, high-level overview of the entire system.

*   **Proposal:** Develop the dashboard to display key metrics and statuses at a glance:
    *   A summary of running vs. stopped instances.
    *   Status indicators for all configured `Providers`.
    *   A feed of the latest critical events (e.g., risk breaches, instance errors).
    *   A high-level portfolio view or profit/loss summary, if the API can provide it.
*   **Benefit:** A well-designed dashboard would give users immediate insight into the health and activity of their strategies without needing to check each section individually.

#### 4. Expand End-to-End Test Coverage

The project is set up for E2E testing with Playwright, but the test suite is small.

*   **Proposal:** Write additional Playwright tests to cover the critical user workflows identified in the analysis, such as:
    *   Creating a provider from start to finish.
    *   Uploading a new strategy module and launching an instance.
    *   Starting and stopping an instance and verifying its status changes.
    *   Updating risk limits and confirming they are applied.
*   **Benefit:** A comprehensive E2E test suite would provide the highest level of confidence that core functionality is working as expected and would be invaluable for preventing regressions as the application evolves.