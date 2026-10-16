# Architecture Blueprint: AgentAd (AAP)

The **Agent Advertising Protocol (AAP)** enables machine-to-machine (M2M) advertising, sponsored tool discovery, and programmatic micro-monetization across autonomous AI agents.

---

## 1. System Topology

```
                  ┌─────────────────────────────────────────┐
                  │               User Intent               │
                  │   ("Book me a flight to San Francisco") │
                  └────────────────────┬────────────────────┘
                                       │
                                       ▼
                  ┌─────────────────────────────────────────┐
                  │          Autonomous Host Agent          │
                  │     (OpenBot / AutoGen / CrewAI)        │
                  └────────────────────┬────────────────────┘
                                       │ Zero-PII Intent Embeddings
                                       ▼
                  ┌─────────────────────────────────────────┐
                  │          AgentAd (AAP) Gateway          │
                  │        (Auction, Guard, Escrow)         │
                  └──────┬───────────────────────────┬──────┘
                         │                           │
          [Real-Time Semantic Bidding]     [Anti-Jailbreak Defense]
                         ▼                           ▼
              ┌─────────────────────┐      ┌───────────────────┐
              │ Advertiser Bidders  │      │ Sanitized Payload │
              │ (United, Delta, Kayak)     │ (Verified MCP Tool)
              └──────────┬──────────┘      └─────────┬─────────┘
                         │                           │
                         └─────────────┬─────────────┘
                                       ▼
                  ┌─────────────────────────────────────────┐
                  │       Cryptographic Settlement          │
                  │   (EIP-712 Receipt & L402 70/20/10 Split)
                  └─────────────────────────────────────────┘
```

---

## 2. Subsystems

1. **Protocol Discovery (`pkg/protocol`)**:
   - Crawls `/.well-known/agent-ads.json` endpoints published by merchants and API service providers.
   - Parses machine-readable schema: bid minimums, target embedding categories, and certified MCP tool manifests.

2. **Semantic Matcher & Real-Time Bidding (`pkg/matcher`, `pkg/auction`)**:
   - Matches intent embeddings against advertiser inventory with low-latency cosine similarity.
   - Executes Vickrey second-price auctions within a <50ms deadline.

3. **Anti-Jailbreak Guard (`pkg/guard`)**:
   - Disarms adversarial instructions disguised as sponsored tool descriptions.

4. **Escrow & Settlement (`pkg/escrow`, `pkg/attest`)**:
   - Generates cryptographically attested Proof-of-Impression and Proof-of-Action receipts.
   - Clears micropayments via EIP-712 structured data or Lightning L402 hashes with programmatic 70/20/10 revenue split.
