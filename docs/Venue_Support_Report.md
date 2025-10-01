# Exchange & Venue Support Report

---

## Executive Summary

This report provides a comprehensive overview of all exchanges and venues currently supported by the project. The coverage spans centralized exchanges (CEX), decentralized exchanges (DEX), institutional venues, market data providers, liquidity providers, and payment infrastructure. Each venue has been evaluated based on integration quality, API reliability, and operational fitness.

---

## Supported Exchanges & Venues

The table below lists **all supported venues** with their category classification and integration assessment. ✓ = supported; — = not supported.

| Venue                 | Category                 | Status | **Integration Quality** | Notes                     |
| --------------------- | ------------------------ | :----: | :---------------------: | ------------------------- |
| **Binance**           | CEX                      |   ✓    |            ✅            |                           |
| **Coinbase**          | CEX                      |   ✓    |            ✅            | Retail API                |
| **Kraken**            | CEX                      |   ✓    |            ✅            |                           |
| **OKX / OKEx**        | CEX                      |   ✓    |            ✅            |                           |
| **Gate.io**           | CEX                      |   ✓    |            ⚠️            |                           |
| **Bitfinex**          | CEX                      |   ✓    |            ✅            |                           |
| **Bitstamp**          | CEX                      |   ✓    |            ✅            |                           |
| **Gemini**            | CEX                      |   ✓    |            ✅            |                           |
| **Bitflyer**          | CEX                      |   ✓    |            ⚠️            |                           |
| **Huobi**             | CEX                      |   ✓    |            ⚠️            |                           |
| **BitMEX**            | CEX (derivatives)        |   ✓    |            ✅            |                           |
| **Bittrex**           | CEX                      |   ✓    |            ❓            | Status varies by region   |
| **HitBTC**            | CEX                      |   ✓    |            ⚠️            |                           |
| **Biki**              | CEX                      |   ✓    |            ❓            |                           |
| **BKEX**              | CEX                      |   ✓    |            ❓            |                           |
| **BTCBox**            | CEX                      |   ✓    |            ⚠️            | JP region                 |
| **Coincheck**         | CEX                      |   ✓    |            ✅            | JP region                 |
| **Indodax**           | CEX                      |   ✓    |            ✅            | ID region                 |
| **Zaif**              | CEX                      |   ✓    |            ⚠️            | JP region                 |
| **BVNEX**             | CEX                      |   ✓    |            ❓            |                           |
| **Liquid**            | CEX                      |   ✓    |            ❓            |                           |
| **ProBit**            | CEX                      |   ✓    |            ⚠️            |                           |
| **Quasar**            | CEX                      |   ✓    |            ❓            |                           |
| **Deribit**           | CEX (options)            |   ✓    |            ✅            | via metcoin‑exch          |
| **LMAX**              | Institutional venue      |   ✓    |            ✅            | via metcoin‑exch          |
| **Uniswap**           | DEX                      |   ✓    |            ✅            | via metcoin‑exch          |
| **CherrySwap**        | DEX                      |   ✓    |            ❓            |                           |
| **OKDEX**             | Provider                 |   ✓    |            ❓            | Market/data infra         |
| **Jupiter (MDP)**     | Provider                 |   ✓    |            ✅            | Market data / routing     |
| **Paradigm (MDP)**    | Provider                 |   ✓    |            ✅            | Block/RFQ & market data   |
| **Circle**            | Provider                 |   ✓    |            ✅            | Fiat rails / USDC         |
| **Delta**             | Provider                 |   ✓    |            ❓            | Market/data infra         |
| **B2C2**              | Liquidity provider       |   ✓    |            ✅            | OTC/MM                    |
| **Cumberland**        | Liquidity provider       |   ✓    |            ✅            | OTC/MM                    |
| **DVChain**           | Liquidity provider       |   ✓    |            ✅            | OTC/MM                    |
| **OSL**               | Liquidity provider / CEX |   ✓    |            ✅            |                           |
| **Signet**            | Payments                 |   ✓    |            ✅            | Bank payment network      |
| **Zing**              | Liquidity / Provider     |   ✓    |            ❓            |                           |

**Legend**  
**Integration Quality**: ✅ solid docs/APIs & support; ⚠️ mixed or venue‑specific caveats; ❓ needs review.

---

## Coverage Summary

The project currently supports **37 venues** across multiple categories:

* **Centralized Exchanges (CEX)**: 23 venues including major global exchanges and regional platforms
* **DEX (Decentralized)**: 2 venues (Uniswap, CherrySwap)
* **Institutional Venues**: 1 venue (LMAX)
* **Market Data Providers**: 5 venues (OKDEX, Jupiter, Paradigm, Circle, Delta)
* **Liquidity Providers**: 5 venues (B2C2, Cumberland, DVChain, OSL, Zing)
* **Payment Infrastructure**: 1 venue (Signet)

**Integration Quality Distribution**:
* ✅ High Quality: 18 venues (49%)
* ⚠️ Moderate Quality: 10 venues (27%)
* ❓ Under Review: 9 venues (24%)

---

## Venue Evaluation Framework

This framework is used to assess integration quality and prioritize engineering effort for new and existing venue integrations.

### A) Core Evaluation Dimensions

1. **API Quality & Stability**: docs completeness, versioning/semver, deprecations, SDKs, sandbox parity.
2. **Market/Feature Coverage**: spot/derivatives/options, margin/borrow‑lend, fiat on/off‑ramp, sub‑accounts.
3. **Market Data**: depth (L2/L3), trade feed, klines, sequencing/latency, gap‑fill tools, historical backfill.
4. **Trading Features**: order types (limit/market/IOC/FOK/POST‑Only/stop/oco), batch/cancel‑all, clientOrderId/idempotency.
5. **Performance & Rate Limits**: documented limits, burst vs sustained, reset behavior, WebSocket throughput.
6. **Reliability & Incident History**: uptime/SLA, major outages, maintenance comms, change‑management cadence.
7. **Integration Complexity**: auth/signing quirks, timestamp skew, precision/lot/tick rules, symbol churn/mapping.
8. **Operational Fit**: KYC/geo access, legal posture, treasury/withdrawals stability, fee schedule/rebates.
9. **Support & Ecosystem**: technical contacts, ticket SLA, community/forums, sample code.
10. **Security Posture**: past breaches, proof‑of‑reserves or attestations, 2FA/API key scopes, IP allowlists.

### B) Scoring Rubric (0–5 per dimension)

| Score | Description                                                                                              |
| ----- | -------------------------------------------------------------------------------------------------------- |
| **5** | Best‑in‑class; zero blockers; clean docs; stable WS with sequencing; rich order set; responsive support. |
| **4** | Strong; minor caveats or rare incidents; straightforward integration.                                    |
| **3** | Adequate; a few gaps/workarounds needed; acceptable reliability.                                         |
| **2** | Weak; frequent quirks or throttling; missing key features.                                               |
| **1** | Poor; unstable or inconsistent; major missing features.                                                  |
| **0** | Not usable now.                                                                                          |

**Suggested Weights** (sum to 1.0): API 0.2, Market/Feature 0.15, Market Data 0.15, Trading 0.15, Performance 0.1, Reliability 0.1, Integration 0.05, Operational 0.05, Support 0.03, Security 0.02.

**Composite Score** = Σ(scoreᵢ × weightᵢ). Document one‑line evidence per dimension.

### C) Mapping Scores to Integration Quality Ratings

* **Integration Quality**:
  * **✅** if Composite ≥ 4.0 and no critical risks.
  * **⚠️** if 3.0–3.9 or notable caveats (regulatory/geo, throttling, flaky WS).
  * **❌** if < 3.0 or critical blockers.
  * **❓** if not yet assessed.

### D) Evaluation Checklist (quick field guide)

* [ ] Docs cover auth, rate limits, order types, errors; changelog exists.
* [ ] Testnet available and near‑parity; sample API keys acquired.
* [ ] REST + WS smoke tests pass; sequencing & gap‑fill verified.
* [ ] Symbol/precision mapper validated; rounding rules unit‑tested.
* [ ] Rate‑limit strategy tuned; retries/backoff configured.
* [ ] Error taxonomy mapped; idempotency supported.
* [ ] Withdrawal/deposit flows tested; fees/rebates understood.
* [ ] Support contact verified; escalation path known.
* [ ] Security controls (IP allowlist, key scopes) enabled.
* [ ] Compliance/KYC/geo constraints documented.

> Tip: keep a per‑venue markdown file with the scores, evidence links, and gotchas; update after every incident or API change.

