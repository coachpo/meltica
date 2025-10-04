# System Architecture Overview

The system is structured into three logical layers — **Level 1**, **Level 2**, and **Level 3** — each responsible for a specific aspect of communication and business processing.

---

## **Level 1 – Connection Layer**

**Purpose:**  
Provides the fundamental infrastructure for network communication.

**Responsibilities:**
- Manages physical connections for both **WebSocket** and **REST** clients.  
- Handles connection lifecycle operations: **connect**, **reconnect**, and **ping/pong** (for WebSocket).  
- Acts as the entry and exit point for all inbound and outbound data.

---

## **Level 2 – Routing Layer**

**Purpose:**  
Handles message routing, translation, and request/response formatting between the connection layer and business layer.

**Responsibilities:**
- **WebSocket:**
  - Manages **subscriptions** and **unsubscriptions**.
  - Routes incoming messages to the correct business handler.
- **REST:**
  - Builds and formats **HTTP requests** (URL, path, headers, payload).
  - Converts raw **HTTP responses** into a standardized internal format for business processing.

---

## **Level 3 – Business Layer**

**Purpose:**  
Implements the domain and business logic.

**Responsibilities:**
- Generates business-level requests and passes them to **Level 2**.  
- Processes normalized responses and messages received from **Level 2**.  
- Distributes results to client interfaces, services, or other consumers.

---

## **Data Flow Summary**

| Communication Type | Level 1 | Level 2 | Level 3 | Description |
|--------------------|----------|----------|----------|--------------|
| **WebSocket** | Connection management (connect, reconnect, ping/pong) | Message routing, subscription/unsubscription | Business logic, request generation | Continuous, bidirectional stream |
| **REST (HTTP)** | Connection handling | Request building & response normalization | Business logic, request initiation | Stateless, point-to-point request/response |
