# Changelog

All notable changes to **AgentAd (AAP)** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]
### Added
- **Real-Time Telemetry & Prometheus Metrics Exporter (`pkg/telemetry`)**: Production-grade concurrent metrics registry and observability engine tracking stage-by-stage latency percentiles (P50, P90, P95, P99), SLA compliance (<50ms target), second-price auction clearing distributions, and 70/20/10 revenue accounting (`agentad telemetry`).
- **Prometheus Text Exposition Format (`RenderPrometheus`)**: Standard Prometheus `# HELP` and `# TYPE` exposition with label vectors (`aap_pipeline_requests_total`, `aap_pipeline_duration_ms`, `aap_stage_duration_ms`, `aap_clearing_price_cpm`, `aap_revenue_usd_total`, `aap_security_threats_blocked_total`, `aap_placement_suppressions_total`).
- **Telemetry CLI & Observability Scorecards (`cmd/agentad/telemetry_snippet.go`)**: Multi-scenario workload simulation, ANSI terminal scorecards (`RenderTelemetryScorecard`), Markdown reports (`RenderTelemetryMarkdown`), and Mermaid architecture diagrams (`--demo`, `--prometheus`, `--json`, `--format`, `--mermaid`, `--strict`).
- **Zero-Jailbreak End-to-End 12-Stage Orchestration Pipeline (`pkg/pipeline`)**: High-performance unified execution pipeline orchestrating traffic scrubbing, Zero-PII sanitization, vector matching, brand safety, dynamic reserve pricing, Vickrey RTB auction, quality scoring (AQS), PID pacing, placement quiet zones, AST factsheet purification, Ed25519 PoI attestation, and 70/20/10 micro-escrow settlement in sub-millisecond latency (`agentad pipeline`).
- **Pipeline CLI Subcommands & Diagnostics (`cmd/agentad/pipeline_snippet.go`)**: Interactive ANSI terminal scorecard (`RenderPipelineScorecard`), Mermaid sequence diagrams (`RenderPipelineMermaid`), and structured Markdown audit reporting (`--demo`, `--query`, `--category`, `--stage`, `--tokens`, `--format`, `--mermaid`).
### Added
- **Supply-Side Manifest Crawler, Anti-SSRF Resolver & Publisher Registry (`pkg/discovery`)**: Automated discovery crawler and thread-safe registry resolving `/.well-known/agent-ads.json` with strict SSRF defense, IP pinning, conditional 304 revalidation, and multi-criteria ad slot querying (`agentad discovery`).
- **SSRF & DNS Rebinding Hardening (`ssrf.go`)**: Restricts private CIDR blocks (RFC 1918, link-local metadata 169.254.0.0/16, loopback) and enforces IP pinning in custom `DialContext` to eliminate TOCTOU DNS rebinding exploits.
- **Discovery CLI Subcommands & Telemetry (`cmd/agentad`)**: Interactive terminal scorecards (`RenderDiscoveryScorecard`), Mermaid resolution flowcharts (`RenderDiscoveryMermaid`), and Markdown reports (`--demo`, `--domain`, `--context`, `--floor`, `--format`).

### Added
- **Contextual Multi-Armed Bandit & Bayesian Thompson Sampling Engine (`pkg/bandit`)**: High-performance exploration/exploitation engine resolving the advertiser cold-start starvation dilemma and optimizing expected revenue ($eCPM = pCTR \times pCVR \times Bid \times Relevance$) in real-time (`agentad bandit`).
- **Thread-Safe Marsaglia-Tsang Gamma & Beta Sampler (`sampler.go`)**: High-precision pseudo-random variate generator sampling $\theta_i \sim \text{Beta}(\alpha_i, \beta_i)$ with Box-Muller standard normal variates for true Bayesian posterior updates.
- **Multi-Algorithm Exploration Suite**: Dynamic support for Bayesian Thompson Sampling, Upper Confidence Bound 1 (UCB1: $\mu_i + c \sqrt{\frac{2 \ln N}{n_i}}$), and $\epsilon$-Greedy exploration strategies.
- **Cold-Start Fair Discovery & Prior Seeding**: Automatic cold-start phase transition ($n < 10$ impressions) prioritizing newcomer tools over monopolistic incumbents.
- **Time-Decayed Feedback Learning**: Closed-loop posterior parameter updates ($\alpha \leftarrow \alpha + 1$ on click, $\alpha \leftarrow \alpha + 2$ on conversion, $\beta \leftarrow \beta + 1$ on drop) with exponential time-discounting ($\lambda = 0.995$) to adapt to seasonal advertiser CTR fluctuations.
- **CLI Subcommand & Scorecard Visualizations**: Terminal ANSI scorecards (`RenderBanditScorecard`), Mermaid decision flowcharts (`GenerateBanditMermaid`), and Markdown simulation reports (`--demo`, `--mermaid`, `--format`).
- **Dynamic Publisher Floor Price Optimizer & Buyer Bid Shading Engine (`pkg/pricing`)**: Real-time publisher yield management calculating dynamic reserve prices based on historical inventory fill rates, category competition density, and agent reputation scores (`agentad floor`).
- **Inventory Fill Rate Adaptation**: Closed-loop feedback mechanism dynamically stiffening reserve floors during high-demand surges and softening floors during low-fill periods to eliminate unsold inventory drops.
- **Buyer-Side Bid Shading & Winner's Curse Protection**: Multi-algorithmic bid shading (`vickrey_emulator`, `winrate_optimizer`, `floor_aware_hybrid`) shaving bids to preserve buyer surplus while enforcing dynamic reserve safety margins.
- **Multi-Vector Candidate Gatekeeping**: Quality-weighted candidate ranking with explicit rejection categorization (`BELOW_DYNAMIC_FLOOR`, `LOW_RELEVANCE`, `LOW_TRUST_REPUTATION`).
- **Terminal Scorecard & Mermaid Visualization**: Comprehensive ANSI terminal scorecards (`RenderPricingScorecard`), aggregate simulation benchmarks (`RenderSimulationSummary`), Mermaid reserve clearance flowcharts, and Markdown export (`--demo`, `--shading`, `--mermaid`, `--format`).
- **Multi-Touch Conversion Attribution & Proof-of-Conversion Engine (`pkg/tracker`)**: Comprehensive attribution engine for autonomous AI-to-AI commerce with mathematical credit distribution across 5 models (`first_touch`, `last_touch`, `linear`, `time_decay`, `position_based`).
- **Cryptographic Proof-of-Conversion (PoC)**: Ed25519 digital signatures linking conversion ID, session context, touchpoints, conversion value, and nonces for tamper-proof merchant attestation.
- **Attribution Governance & Windowing**: Configurable lookback conversion window (default 24h), cryptographic replay attack rejection, and idempotent conversion registration.
- **Automated Multi-Party Bounty Settlement**: CPA/CPS reward splits (70% publisher, 20% host, 10% protocol) distributed proportionally based on attribution credits.
- **CLI Subcommand & Reporting (`agentad track`)**: Interactive terminal scorecard (`RenderTrackerScorecard`), Mermaid sequence diagrams (`RenderMermaid`), and Markdown summary tables (`--demo`, `--model`, `--mermaid`, `--format`).
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
