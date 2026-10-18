# Changelog

All notable changes to **AgentAd (AAP)** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- **Click Fraud, Synthetic Traffic Scrubbing & Prompt Leakage Defense Engine (`pkg/fraud`)**: Real-time traffic scoring and telemetry sanitization pipeline protecting advertisers and publishers against automated agent fraud and sensitive context leakage.
- **Shannon Entropy Anomaly Scoring ($H(X)$)**: Statistical entropy calculation detecting low-entropy synthetic bot traffic, query flooding, and repetitive automated scraping.
- **Multi-Vector Threat Classification**: Real-time identification of 6 attack vectors: `SYNTHETIC_QUERY_FLOODING`, `KEYWORD_STACKING`, `PROMPT_LEAKAGE_PROBE`, `SYBIL_AGENT_RECYCLING`, `UNNATURAL_BURST_PACING`, and `CANARY_TOKEN_EXFILTRATION`.
- **Sliding-Window Velocity & Sybil Tracking**: Rolling 60-second window monitoring agent velocity (RPM) and duplicate query counts with configurable threat thresholds.
- **Canary Token Exfiltration & Leakage Traps**: Dynamic honey-token matching and prompt-leakage probe heuristics blocking reconnaissance queries targeting internal system instructions or private keys.
- **CLI Subcommand & Scorecards (`agentad fraud`)**: Rich ANSI terminal telemetry inspection and Mermaid triage graph generation (`--demo`, `-strict`, `--mermaid`).
- **Semantic Vector Matcher & Zero-PII Intent Matching Engine (`pkg/matcher`)**: Sub-millisecond vector indexing and retrieval engine performing real-time cosine similarity search across advertiser target inventories.
- **Zero-PII Cryptographic Intent Commitments**: Generates SHA-256 salted intent commitment hashes protecting raw user prompts from advertiser exfiltration while enabling verifiable audit trails.
- **Deterministic Token-Shingle Vector Projector**: Sub-millisecond zero-dependency embedding generation projecting freeform queries and advertiser inventory into 384-dimensional unit vectors.
- **CLI Subcommand & Visualizations (`agentad match`)**: Terminal ANSI scorecard formatting (`RenderMatchScorecard`) and Mermaid pipeline flowcharts (`RenderMatchMermaid`).
- EIP-712 structured data signing for EVM settlement receipts and Proof-of-Impression claims.
- Lightning L402 / HTTP 402 payment hashes with automated multi-party escrow clearing (70/20/10 split).
- Anti-jailbreak prompt injection guardrail scanner with delimiter stripping.
- Real-Time Bidding (RTB) Vickrey second-price auction engine.

---

## [v0.1.0] - 2026-10-16

### Added
- Initial specification for Agent Advertising Protocol (AAP).
- `/.well-known/agent-ads.json` crawler and parser.
- Real-Time Bidding (RTB) semantic auction engine.
- Model Context Protocol (MCP) tool injection proxy.
- Cryptographic attestation and proof-of-impression generator.
