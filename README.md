# Stack Manager

Manage all your Docker Compose stacks from one dashboard. Discover, deploy, update, back up, and monitor projects across a fleet of hosts — with a 200+ template catalog, scheduled updates, live shell access, firewall management, two-factor auth, and agents that phone home from behind NAT.

![Stack Manager login &amp; landing page](docs/images/login-landing-dark.png)

---

## Highlights

- **200+ one-click stack templates** — AI, databases, CMS, monitoring, proxies, dev tools, media, and more. Pick a template, review the compose and env, and spin it up.
- **In-browser config editor** — edit compose.yml, .env, Caddyfile, and other project files directly from the dashboard with automatic .bak backups.
- **Fleet management with agents & peers** — register remote Docker hosts as outbound (phone-home), inbound, or combined agents, or add another full install as a **peer controller**. The "All Servers" view shows every connected host; open and manage their projects, and act across all of them from one controller. Behind-NAT **callback agents** are managed through a command queue that runs on their next check-in. All cross-server traffic uses TLS 1.3.
- **GPU for AI stacks** — Settings > GPU detects the host GPU, one-click-installs the NVIDIA driver + toolkit, and runs a real `--gpus all` test container (nvidia-smi) to prove passthrough works. "Add GPU passthrough" is a checkbox on both the Stack Catalog and the Create Project form (baked in before deploy), or an **Enable GPU** action after the fact.
- **Per-stack volumes & networks** — inspect a project's Docker volumes and networks (with in-use containers) and safely delete them, scoped to that stack, from its detail page.
- **One-click self-update** — Settings > Update pulls and rebuilds the controller on the host (detached, survives the restart) and shows **what's in the update** (the pending commit subjects) before you run it.
- **Scheduled updates** — daily at 03:00, weekly on Saturday, monthly on the 15th, or every N minutes. Per-project update policies prevent accidental breakage.
- **Backup to anywhere** — local paths, CIFS, NFS, FTP, SFTP (with in-browser SSH key generation), and S3. Automatic local archive + remote copy.
- **Live in-browser shell** — xterm.js terminal with real PTY support that opens an interactive session inside any running container via WebSocket + docker exec. Tab completion, arrow keys, colors, and resize — as fast as SSH.
- **Firewall management** — install, configure, and monitor ConfigServer Firewall (csf/lfd) from the dashboard with structured port forms, testing-mode toggle, allow/deny lists, config editor, and live log viewer. Login IPs are auto-allowlisted. Docker compatibility is auto-configured.
- **Reverse proxy integration** — one-click **deploy** Nginx Proxy Manager, then add proxied domains from the dashboard: per-project **Add to Proxy**, a one-click **proxy the Stack Manager UI** target, and auto-filled forwards for running projects. Let's Encrypt stays separate for non-proxy installs.
- **Two-factor authentication** — TOTP (Google Authenticator / Authy) with QR enrollment, backup codes, and per-user enable/disable.
- **Self-signed TLS out of the box** — HTTPS on first boot with zero config. Optional Let's Encrypt or Nginx Proxy Manager for real domains.
- **General settings in the browser** — change ports, cache TTLs, host URL, a friendly **server display name** (shown in the server selector instead of the IP), the **timezone** (which sets the host system clock via `timedatectl` so all containers follow it), and roll the API key from Settings > General without touching .env or SSH.
- **Multiple Docker roots** — discover projects across more than one host directory via `EXTRA_DOCKER_ROOTS`.
- **Docker daemon settings** — edit `daemon.json` from the browser with tooltips, backups, and teardown guidance for network changes.
- **Security scans** — image vulnerability scanning and compose audit from the project detail page.
- **Database checks** — health checks and SQL dumps for MariaDB, MySQL, and PostgreSQL containers.
- **Audit log** — every mutating action recorded with actor, project, result, and timestamp, with quick "Updates run" / "Backups run" presets and an **Activity log** link from each project that deep-links to its entries.

---

## Quick Start

```bash
mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

Open the URL printed by the setup script (default `https://<your-ip>:8993`). All passwords and keys are auto-generated on first run.

---

## Screenshots

Existing catalog screenshots:

![AI Stack Catalog](docs/images/catalog-personal-ai-agents-dark.png)
![Web Catalog](docs/images/catalog-web-dark.png)
![CMS Catalog](docs/images/catalog-cms-dark.png)
![Database Catalog](docs/images/catalog-database-light.png)
![Queue Catalog](docs/images/catalog-queue-light.png)
![Dev Tools Catalog](docs/images/catalog-devtools-light.png)

![Project Detail — Backups](docs/images/project-detail-backups.png)

<!-- Add more screenshots as you take them:
![Dashboard](docs/images/dashboard-dark.png)
![Project Detail](docs/images/project-detail.png)
![Settings General](docs/images/settings-general.png)
![Firewall Panel](docs/images/firewall-panel.png)
![Interactive Shell](docs/images/interactive-shell.png)
![Config Editor](docs/images/config-editor.png)
![TOTP Enrollment](docs/images/totp-enrollment.png)
![Reverse Proxy](docs/images/reverse-proxy.png)
![Scheduled Updates](docs/images/scheduled-updates.png)
-->

---

## Features in Detail

### Dashboard

The main page shows every discovered Compose project with live state, image sources, update availability, and one-click actions: start, stop, restart, update, pull, backup, and delete. Filter by running, stopped, inactive, or projects with available updates. Bulk actions apply to the filtered list or a manual selection.

### Stack Catalog (275 Templates)

Browse and deploy from a curated catalog organized into 20 categories and 10 AI subcategories:

| Category | Examples |
|----------|----------|
| **AI** | Ollama + Open WebUI, OpenBrain agent stacks (workflow / memory / visual builder), Voice Assistant (Ollama + Open WebUI + Kokoro TTS, voice pre-wired), LibreChat, AnythingLLM, ComfyUI, Whisper, Langfuse |
| **Web & Proxy** | Nginx, Caddy, Traefik, Nginx Proxy Manager, Apache, ProxyForge (SOCKS5 + HTTP proxy admin) |
| **CMS** | WordPress, Ghost, Strapi, Directus, Payload, Concrete CMS |
| **Database** | PostgreSQL, MariaDB, Redis, MongoDB, CockroachDB, ClickHouse |
| **Dev Tools** | Gitea, Forgejo, code-server, Jenkins, Drone CI, Draw.io |
| **Monitoring** | Grafana + Prometheus, Uptime Kuma, Beszel, Netdata, cAdvisor, Umami (analytics) |
| **Docs** | BookStack, Docmost, DokuWiki, Wiki.js, Outline |
| **Media** | Jellyfin, Plex, Radarr, Sonarr, Prowlarr, qBittorrent, Jellyseerr, Immich, PhotoPrism, Bambuddy, Maintainerr, Tube Archivist, ErsatzTV, Unpackerr |
| **Gaming** | EmulatorJS, RomM, Sunshine (Moonlight game streaming) |
| **Remote** | Webtop (full Linux desktop in browser), Neko (shared virtual browser), Apache Guacamole, RustDesk, Firefox |
| **Security** | Authelia, Authentik, Keycloak, CrowdSec, Vaultwarden, AdGuard Home, Pi-hole, ARPVPN, wg-easy, WireGuard-UI, Headscale, Pritunl |
| **Files** | Nextcloud, Seafile, MinIO, Paperless-ngx, ConvertX |
| **Mail** | Stalwart Mail Server (SMTP/IMAP/POP3/JMAP) |
| **Finance** | Actual Budget, Firefly III, Wallos |
| **Productivity** | Karakeep, Linkwarden, Memos, Vikunja, FreshRSS, Planka |
| **Queue** | RabbitMQ, Apache ActiveMQ, Beanstalkd, Faktory |
| **Automation** | n8n, Huginn, Cronicle, Changedetection.io, ntfy |
| **Chat** | The Lounge, Ergo, ZNC, Convos (IRC), Matrix Synapse + Element, continuwuity, Mattermost, Rocket.Chat, Prosody, ejabberd (XMPP), Chatwoot |
| **Management** | Homepage, Dashy, Homarr, Portainer, Dockge, Yacht, Watchtower |

Templates load into an editable Create Project form — review ports, volumes, passwords, and env vars before deploying. Nothing deploys until you click Create. Templates that need a config file ship a working starter config embedded in the compose (via `configs:`), so they boot out of the box and you can edit the config any time from a project's Config tab.

<details>
<summary><b>Full catalog — all 275 apps</b> (click to expand)</summary>


#### AI & Machine Learning (77)


*code assistants*

- **Aider CLI Container** — Terminal-based AI pair programmer packaged as a long-lived shell container.
- **code-server + Ollama** — VS Code in the browser with a local Ollama sidecar for AI extensions.
- **Ollama + Code Llama** — Ollama preloaded with a Code Llama model for local IDE integrations.
- **OpenHands (formerly OpenDevin)** — Autonomous coding agent that plans and executes tasks.
- **Refact Self-Hosted** — Open-source coding assistant with fine-tuning and RAG.
- **SGLang Code Server** — SGLang inference server for code models (OpenAI-compatible API).
- **Tabby** — Self-hosted AI coding assistant server.

*evals*

- **promptfoo** — Self-hosted prompt, agent, RAG, and model evaluation UI for private eval sharing and red-team workflows.

*image generation*

- **AUTOMATIC1111** — Stable Diffusion web UI starter.
- **ComfyUI** — Node-based Stable Diffusion image generation UI.
- **Fooocus** — Streamlined Stable Diffusion UI focused on ease of use.
- **InvokeAI** — Stable Diffusion creative studio with node graph and gallery UI.
- **rembg (background removal)** — Web/API server for removing image backgrounds with u2net.
- **SD WebUI Forge** — AUTOMATIC1111 fork optimized for speed and memory.

*llm inference*

- **AnythingLLM** — Private AI workspace with document chat and agents.
- **LiteLLM Proxy** — OpenAI-compatible proxy for many LLM providers.
- **LobeChat** — Polished multi-provider AI chat interface.
- **LocalAI** — OpenAI-compatible local model API.
- **Ollama** — Local LLM model runner API.
- **Ollama + Open WebUI** — Local AI model runner with a web chat interface.
- **Open WebUI** — Web chat interface for Ollama and OpenAI-compatible APIs.
- **Text Generation Inference** — Hugging Face LLM inference server.
- **vLLM OpenAI Server** — High-throughput OpenAI-compatible LLM inference server.

*observability*

- **Arize Phoenix** — Open-source AI observability and evaluation platform for traces, datasets, experiments, prompt management, and playgrounds.
- **Langfuse** — Open-source LLM observability, tracing, prompt management, metrics, datasets, playground, and evaluations platform.

*personal agents*

- **Hermes Agent** — Autonomous AI agent gateway by NousResearch with OpenAI-compatible API, web dashboard, and multi-channel support.
- **Khoj** — Self-hostable personal AI second brain for docs, web answers, custom agents, automations, and local or hosted LLMs.
- **Letta Agent** — Stateful agent platform with advanced memory, self-improvement, tools, subagents, and local or self-hosted server options.
- **Moltworker** — Cloudflare-maintained OpenClaw worker/sandbox gateway for customers who want a Cloudflare-edge deployment path.
- **NanoClaw** — Lightweight secure OpenClaw-style agent aimed at simple personal and team deployments with isolated execution.
- **OpenClaw** — Popular local-first personal AI assistant with a gateway for WhatsApp, Telegram, Slack, Discord, Signal, iMessage and many other channels.
- **ZeroClaw** — Small, security-first personal agent runtime with supervised autonomy, workspace boundaries, command policy, and OS/container sandbox options.

*search*

- **Crawl4AI** — Open-source LLM-friendly crawler and scraper with secure-by-default Docker API server for Markdown/RAG extraction.
- **Elasticsearch (single-node)** — Elasticsearch single-node cluster for development or small workloads.
- **Firecrawl** — Self-hosted web context API for search, scrape, crawl, extract, and agent-ready clean Markdown/structured data.
- **Manticore Search** — SphinxSearch fork with full-text, vector, and SQL support.
- **Meilisearch** — Lightning-fast typo-tolerant search engine.
- **OpenSearch (single-node)** — OpenSearch single-node cluster for full-text and vector search.
- **Perplexica** — Open-source alternative to Perplexity AI (SearXNG + LLM).
- **SearXNG** — Private metasearch engine useful for AI research workflows.
- **Sonic** — Fast, lightweight schema-less search backend.
- **Typesense** — Open-source, faceted search server with typo tolerance.
- **ZincSearch** — Lightweight Elasticsearch alternative in a single binary.

*vector db*

- **Chroma** — Embeddings database for local AI apps.
- **Marqo** — End-to-end tensor search engine (vector + BM25).
- **Milvus Standalone** — High-performance open-source vector database (standalone mode).
- **OpenSearch (vector-ready)** — OpenSearch single-node with the k-NN plugin for vector search.
- **Postgres + pgvector** — PostgreSQL with the pgvector extension for vector similarity search.
- **Qdrant** — Vector database for semantic search and RAG.
- **Redis Stack** — Redis with RediSearch, RedisJSON, and vector search modules.
- **Vespa** — Yahoo/Verizon Media's search + vector engine.
- **Weaviate** — Vector database with optional module support.

*voice speech*

- **Coqui TTS** — Coqui TTS server with a REST API for many voices.
- **faster-whisper Server** — GPU-optimized Whisper reimplementation with an OpenAI-compatible API.
- **OpenBrain #4 — Voice Assistant (speech, web, code, memory)** — The do-everything local AI: talk to it (Whisper STT + Kokoro/Piper TTS), search the web (SearXNG), write code (Code Llama), keep memory (mem0 + Qdrant), and automate tasks (n8n + Flowise) — all wired to Open WebUI and Ollama.
- **openedai-speech (TTS + voice cloning)** — OpenAI-compatible text-to-speech: fast Piper voices plus XTTS-v2 voice cloning from a reference sample. Drop-in TTS for Open WebUI.
- **Piper TTS (Wyoming)** — Fast local text-to-speech, exposed over the Wyoming protocol.
- **Rhasspy Voice Assistant** — Fully offline private voice assistant with web UI, wake word, speech-to-text, intent recognition, and text-to-speech.
- **Voice Assistant (Ollama + Open WebUI + Kokoro TTS)** — Talk to a local LLM by voice. Open WebUI is pre-wired to Kokoro for text-to-speech and its built-in Whisper for speech-to-text — voice works on first login, no manual audio setup.
- **Whisper ASR** — Speech-to-text API using Whisper models.
- **whisper.cpp Server** — C++ inference server for OpenAI Whisper models.
- **XTTS v2 Server** — Coqui XTTS-v2 streaming voice cloning server.

*workflow rag*

- **Dify** — LLM app development platform (studio + orchestration).
- **DocsGPT** — Private AI platform for document assistants, enterprise search, agent builder, document analysis, and multi-model chat.
- **Flowise** — Visual builder for AI chains, agents, and chatflows.
- **Haystack REST API** — deepset Haystack pipeline server with an OpenAPI interface.
- **JupyterLab PyTorch** — Notebook workspace with PyTorch for AI experiments.
- **Langflow** — Visual LangChain-style AI app builder.
- **LibreChat** — Self-hosted multi-provider ChatGPT-style platform with agents, MCP, files, search, artifacts, and multi-user auth.
- **Onyx** — Open-source enterprise AI chat and knowledge platform with connectors, search, agents, and every-LLM support.
- **Open WebUI Pipelines** — Plugin and function server for Open WebUI.
- **OpenBrain #1 — Agent + Workflow** — Ollama + Open WebUI + Postgres + Neo4j + Temporal for durable agent workflows with graph knowledge.
- **OpenBrain #2 — Agent + Memory + Automation** — Ollama + Open WebUI + Qdrant + mem0 (OpenMemory) + Neo4j + n8n for agents with durable vector memory and workflow automation.
- **OpenBrain #3 — Visual Agent Builder** — Ollama + Open WebUI + Flowise + Qdrant + Postgres for building low-code LLM agents and RAG chains visually.
- **OpenMemory + Mem0** — Self-hosted Mem0/OpenMemory stack with an MCP memory API, web UI, and Qdrant vector storage.
- **RAGFlow** — Enterprise RAG platform with document parsing pipelines.
- **Verba (Weaviate RAG)** — Weaviate's RAG chatbot with a polished UI.

#### Web Servers (10)

- **Apache HTTP Server** — Apache static web server with a bind-mounted public directory.
- **Caddy Static + Auth** — Caddy 2 serving files with basicauth from Caddyfile.
- **Caddy Static Site** — Caddy web server for static files.
- **Hugo Static Server** — Hugo dev server with live reload for a static site.
- **Nginx Static Site** — Small static web server with a bind-mounted html directory.
- **Nginx Unprivileged** — Rootless-friendly Nginx static web server.
- **OpenResty** — Nginx + LuaJIT distribution for scripted request handling.
- **PHP Apache** — PHP-enabled Apache web app starter.
- **Shlink** — Self-hosted, API-driven URL shortener with detailed click analytics and QR codes.
- **Traefik Whoami** — Tiny HTTP echo service for route and proxy testing.

#### Proxy (10)

- **Caddy** — Simple HTTPS-ready web reverse proxy.
- **Envoy Proxy** — Cloud-native L4/L7 proxy for service meshes and edge.
- **HAProxy** — Reliable, high-performance L4/L7 load balancer.
- **Nginx Proxy Manager** — GUI reverse proxy with free SSL via Let's Encrypt. Manage proxy hosts, redirections, streams, and certificates from a browser.
- **Pomerium** — Identity-aware access proxy for internal services.
- **ProxyForge** — ARPHost's multi-tenant web admin for SOCKS5 (Dante) + HTTP CONNECT (Squid) proxies, one credential for both.
- **Squid Proxy** — Caching HTTP forward proxy.
- **SWAG (LinuxServer.io)** — Reverse proxy with automatic Let's Encrypt certs and fail2ban.
- **Traefik** — Docker-aware reverse proxy with dashboard.
- **Zoraxy** — All-in-one reverse proxy with GUI, TLS, and IP filtering.

#### CMS (8)

- **Directus** — Headless CMS and API layer backed by PostgreSQL.
- **Drupal + PostgreSQL** — Drupal CMS with PostgreSQL persistence.
- **Ghost + MySQL** — Ghost publishing CMS with MySQL persistence.
- **Grav** — Flat-file CMS with no database dependency.
- **Joomla + MariaDB** — Joomla CMS with MariaDB persistence.
- **PrestaShop** — Open-source e-commerce platform.
- **Strapi Headless CMS** — Open-source headless CMS with an admin panel.
- **WordPress + MariaDB** — Blog/CMS stack with WordPress and MariaDB.

#### Databases (9)

- **Adminer** — Small database administration UI.
- **CockroachDB (single-node)** — Distributed SQL database in single-node insecure mode.
- **MariaDB** — MariaDB 11.4 with persistent volume.
- **MongoDB** — MongoDB document database with root credentials.
- **MySQL** — MySQL database with persistent storage.
- **Neo4j Community** — Graph database with the Neo4j Browser UI.
- **NocoDB** — Spreadsheet-style database UI and no-code app builder.
- **PostgreSQL** — PostgreSQL database with persistent volume.
- **Redis** — Redis cache with append-only persistence and password auth.

#### Dev Tools (11)

- **code-server** — Browser-based VS Code server.
- **CyberChef** — The Cyber Swiss-army knife: encryption, encoding, compression, and data analysis in the browser.
- **draw.io (diagrams.net)** — Self-hosted diagram editor.
- **Drone CI Server** — Container-native CI/CD platform.
- **Excalidraw** — Virtual hand-drawn-style whiteboard for sketching diagrams and wireframes.
- **Forgejo** — Self-hosted Git forge with SSH and web access.
- **Gitea** — Lightweight Git service with SSH and web ports.
- **IT Tools** — Browser toolbox for encoding, crypto, network, and dev utilities.
- **Jenkins** — Automation server for builds and delivery workflows.
- **SonarQube Community** — Code quality and static analysis platform.
- **Woodpecker CI Server** — Lightweight open-source CI/CD server.

#### Monitoring & Analytics (12)

- **Beszel** — Lightweight server monitoring hub with a local agent.
- **cAdvisor** — Container resource usage and performance metrics.
- **changedetection.io** — Website change monitoring.
- **Gatus** — Developer-friendly status page and health monitor.
- **Netdata** — Real-time host and container metrics dashboard.
- **Prometheus + Grafana** — Monitoring starter stack with Prometheus and Grafana.
- **Prometheus Blackbox Exporter** — Probes HTTP, TCP, DNS, and ICMP endpoints for Prometheus.
- **Prometheus Node Exporter** — Host-level metrics exporter for Prometheus.
- **Scrutiny** — Hard-drive SMART monitoring dashboard with historical health/temperature tracking and alerts.
- **Speedtest Tracker** — Scheduled internet speed-test logging with a web dashboard, history, and charts.
- **Umami** — Privacy-friendly, self-hosted web analytics — a lightweight Google Analytics replacement.
- **Uptime Kuma** — Self-hosted uptime monitor.

#### Docs & Wikis (9)

- **BookStack** — Documentation/wiki platform with MariaDB.
- **Docmost** — Open-source collaborative wiki and documentation platform.
- **DokuWiki** — File-backed wiki with no external database.
- **HedgeDoc** — Collaborative markdown notes and documentation editor.
- **MkDocs Material** — Static site generator for technical documentation.
- **Outline Wiki** — Team knowledge base with markdown editor.
- **Paperless-ngx** — Document intake, OCR, tagging, and archive search.
- **Stirling PDF** — Self-hosted PDF merge, split, convert, and repair tools.
- **Wiki.js** — Modern wiki and knowledge base backed by PostgreSQL.

#### Media (39)

- **Audiobookshelf** — Self-hosted audiobook and podcast server.
- **autobrr** — Real-time IRC/RSS filter that grabs torrents and hands releases to your download client.
- **Bazarr** — Subtitle management for Sonarr and Radarr.
- **Calibre-Web** — Web library for ebooks backed by a Calibre database.
- **Cobalt** — Self-hosted social-media/video downloader backend exposing a clean JSON download API.
- **Deluge** — Lightweight BitTorrent client with a web UI.
- **Emby** — Personal media server for video, music, and photos.
- **ErsatzTV** — Builds custom live-TV channels from your library, streamed to Plex/Jellyfin/Emby via HDHR/M3U.
- **FlareSolverr** — Proxy that solves Cloudflare / DDoS-Guard challenges for indexers.
- **Immich** — Self-hosted photo and video backup and management with mobile apps and ML search.
- **Jackett** — Indexer proxy giving Sonarr/Radarr access to torrent and Usenet trackers.
- **Jellyfin** — Self-hosted media server.
- **Jellyseerr** — Media request and discovery for Jellyfin, Plex, and Emby (the maintained Overseerr fork).
- **Jellystat** — Statistics and watch-history dashboard for a Jellyfin server (a Tautulli for Jellyfin).
- **Kavita** — Fast self-hosted library for comics, manga, and ebooks with a web reader.
- **Komga** — Comics, manga, and digital-book server with a web reader and OPDS.
- **Lidarr** — Music collection management and automated downloading.
- **Maintainerr** — Rule-based library maintenance that finds and cleans up stale media in Plex/Jellyfin/Emby.
- **MeTube** — Web GUI for yt-dlp to download video and audio from YouTube and many other sites.
- **Navidrome** — Self-hosted music streaming server.
- **NZBGet** — Efficient Usenet downloader.
- **Ombi** — Request platform for Plex/Emby/Jellyfin users (movies, TV, and music).
- **PhotoPrism** — AI-powered self-hosted photo library — automatic tagging, faces, places, and RAW support.
- **Pinchflat** — Self-hosted YouTube channel/playlist DVR that auto-downloads and archives new videos.
- **Plex Media Server** — Personal media server for streaming libraries.
- **Prowlarr** — Indexer manager and proxy for Sonarr, Radarr, Lidarr, and Readarr.
- **qBittorrent** — BitTorrent client with web UI.
- **Radarr** — Movie library and download automation.
- **Readarr** — Book and audiobook management and automation.
- **Recyclarr** — Syncs TRaSH-Guides custom formats and quality profiles into Sonarr and Radarr on a schedule.
- **SABnzbd** — Free and open-source Usenet downloader.
- **Sonarr** — TV series library and download automation.
- **Tautulli** — Monitoring and tracking for Plex Media Server.
- **Tdarr** — Distributed media transcoding and health-checking (audio/video) with a web UI.
- **Threadfin** — IPTV/M3U proxy and EPG manager that feeds live TV into Plex, Jellyfin, and Emby.
- **Transmission** — Lightweight BitTorrent client with web UI.
- **Tube Archivist** — Self-hosted YouTube media server: subscribe, download, index and stream channels with search.
- **Unpackerr** — Headless daemon that auto-extracts compressed downloads for Sonarr/Radarr/Lidarr/Readarr.
- **Wizarr** — Invitation and user-management portal for onboarding users to Jellyfin, Plex, and Emby.

#### Gaming (3)

- **EmulatorJS** — In-browser retro game emulation with ROM management.
- **RomM** — Self-hosted ROM manager with EmulatorJS play-in-browser, metadata scraping, and save sync.
- **Sunshine** — Self-hosted game streaming server compatible with Moonlight clients.

#### Remote Desktop & Browsers (5)

- **Apache Guacamole** — Clientless RDP/VNC/SSH remote-desktop gateway accessed entirely from the browser.
- **Firefox Browser** — Firefox web browser running in Docker, accessible from the browser.
- **Neko** — Self-hosted virtual browser streamed over WebRTC for shared/collaborative viewing.
- **RustDesk Server** — Self-hosted RustDesk signal + relay server for private remote-desktop connections (TeamViewer alt).
- **Webtop** — Full Linux desktop accessible from the browser (Ubuntu XFCE).

#### Security, Identity & VPN (16)

- **AdGuard Home** — Network-wide DNS ad/tracker blocker with DoH/DoT, DHCP, and per-client filtering — a Pi-hole alternative.
- **ARPVPN** — ARPHost's WireGuard VPN management web GUI — users, roles, live traffic graphs, and a full API.
- **Authelia** — Single sign-on and two-factor authentication portal.
- **Authentik** — Modern identity provider / SSO — SAML, OAuth2/OIDC, LDAP, and forward-auth for reverse proxies.
- **certbot** — Let's Encrypt cert issuer (interactive mode).
- **CrowdSec** — Collaborative intrusion detection and remediation engine.
- **fail2ban** — Log-scanning intrusion prevention.
- **Headscale** — Self-hosted, open-source implementation of the Tailscale control server for your own mesh.
- **Keycloak + PostgreSQL** — Identity and access management server with PostgreSQL.
- **Pi-hole** — DNS sinkhole and local network ad blocker.
- **Pritunl** — Enterprise OpenVPN/WireGuard server with a web admin console (community Docker image + MongoDB).
- **Trivy Server** — Vulnerability scanner server for container and filesystem scans.
- **Vaultwarden** — Lightweight Bitwarden-compatible password vault.
- **Wazuh Manager** — Open-source security monitoring / SIEM manager.
- **wg-easy** — The simplest self-hosted WireGuard VPN with a clean web UI for managing peers.
- **WireGuard-UI** — Web UI for WireGuard paired with the linuxserver/wireguard data-plane container.

#### Files & Storage (10)

- **ConvertX** — Self-hosted file converter supporting 1000+ formats (documents, images, video, audio).
- **Duplicati** — Web-managed encrypted backup tool for files and folders.
- **File Browser** — Web file manager for a mounted directory.
- **MinIO** — S3-compatible object storage with web console.
- **Nextcloud** — File sync and sharing with MariaDB.
- **ownCloud** — Enterprise-flavored file collaboration server.
- **Pydio Cells** — Modern file sharing and collaboration platform.
- **Seafile** — Self-hosted file sync + sharing (community edition).
- **SFTPGo** — Managed SFTP, WebDAV, FTP, and object-storage gateway.
- **Syncthing** — Peer-to-peer file synchronization.

#### Mail (1)

- **Stalwart Mail Server** — All-in-one self-hosted mail server (SMTP, IMAP, POP3, JMAP, ManageSieve) with a web admin.

#### Finance (3)

- **Actual Budget** — Self-hosted envelope/zero-based budgeting app with a fast local-first sync server.
- **Firefly III** — Self-hosted personal finance manager with double-entry accounting, budgets, and reports.
- **Wallos** — Open-source self-hosted personal subscription and recurring-expense tracker.

#### Productivity (7)

- **Dawarich** — Self-hosted location-history tracker and Google Maps Timeline replacement.
- **FreshRSS** — Self-hosted RSS/Atom feed aggregator — fast, multi-user, with an extension system and mobile API.
- **Karakeep** — Self-hosted AI bookmarking (formerly Hoarder) with full-text search and auto-tagging.
- **Linkwarden** — Self-hosted bookmark manager that archives full-page snapshots (screenshot/PDF/HTML) of links.
- **Memos** — Lightweight, markdown-native self-hosted note-taking and microblog tool.
- **Planka** — Self-hosted Trello-style kanban board for project management with real-time collaboration.
- **Vikunja** — Self-hosted to-do list and project management (API + frontend in one image).

#### Message Queues (9)

- **Apache ActiveMQ Classic** — Popular JMS broker with STOMP, AMQP, and MQTT support.
- **Apache Kafka** — Single-node Kafka broker using KRaft mode.
- **Beanstalkd** — Simple fast work queue service.
- **Eclipse Mosquitto** — MQTT broker for IoT and event messaging.
- **Faktory** — Language-agnostic background job server.
- **NATS** — Lightweight messaging system with JetStream persistence.
- **RabbitMQ** — Message broker with management console.
- **Redpanda** — Kafka-compatible streaming platform without ZooKeeper.
- **Temporal (dev)** — Temporal workflow engine dev server with built-in UI.

#### Automation & Notifications (10)

- **Cronicle** — Multi-server cron scheduler with a web UI.
- **Diun** — Docker image update notification service.
- **Gotify** — Self-hosted push notifications server.
- **Home Assistant** — Home automation server.
- **Huginn** — Self-hosted scenario builder for scraping and automation.
- **Mealie** — Recipe manager with meal planning and shopping lists.
- **n8n** — Workflow automation tool.
- **Node-RED** — Flow-based automation and integration builder.
- **ntfy** — Self-hosted pub-sub push-notification server with web UI, REST API, and mobile apps.
- **Watchtower** — Automated container image updater.

#### Chat & Messaging (15)

- **Chatwoot** — Open-source live-chat / customer-support platform (omnichannel help desk).
- **Continuwuity** — Lightweight single-binary Matrix homeserver (community continuation of conduwuit).
- **Convos** — Persistent, always-online IRC web client with a built-in bouncer, in your browser.
- **ejabberd** — Robust, massively-scalable XMPP server with web admin and MUC/PubSub.
- **Element Web** — Full-featured Matrix web client (static SPA) pointed at your homeserver.
- **Ergo** — Modern all-in-one IRCd with bouncer, history, and account services built in.
- **Matrix Synapse** — Reference Matrix homeserver for federated, end-to-end-encrypted chat.
- **Mattermost** — Self-hosted Slack alternative for team messaging (Team Edition, open source).
- **Prosody** — Lightweight, modular XMPP/Jabber server (official Prosody image).
- **Rocket.Chat** — Open-source team chat / Slack alternative with channels, voice, and integrations.
- **Snikket** — All-in-one, opinionated self-hosted XMPP service (server + portal + push + TURN).
- **Soju** — Advanced modern IRC bouncer with multi-user, history, and IRCv3 support.
- **The Lounge** — Self-hosted, always-on web IRC client with a built-in bouncer.
- **UnrealIRCd** — Classic, widely-deployed open-source IRC server (UnrealIRCd 6).
- **ZNC** — Advanced IRC bouncer that stays connected and replays buffers to your client.

#### Management & Dashboards (11)

- **Bambuddy** — Self-hosted command center and print archive for Bambu Lab 3D printers — no cloud, from one A1 to a 40-printer farm.
- **Dashy** — Self-hosted start page and service dashboard.
- **Dockge** — Docker Compose stack manager.
- **Dozzle** — Lightweight Docker log viewer.
- **Homarr** — Modern, drag-and-drop dashboard for your self-hosted services with 40+ live integrations.
- **Homepage** — Clean self-hosted dashboard for services and widgets.
- **Portainer Agent** — Portainer agent for hosts that also need Portainer compatibility.
- **Portainer CE** — Docker and Compose management UI.
- **Rundeck** — Job scheduler and runbook automation.
- **Semaphore UI** — Web UI for running Ansible playbooks, Terraform, and shell scripts.
- **Yacht** — Web UI for managing Docker containers with app templates.

</details>


### In-Browser Config Editor

The **Config** tab on each project detail page lets you edit compose.yml, .env, Dockerfile, Caddyfile, nginx.conf, and other config files directly from the browser. Every save creates a `.bak` backup. A hint reminds you to restart the stack after compose changes.

### Scheduled Updates

Set human-readable schedules from Settings or the Dashboard:

- **Daily** at a specific time (UTC)
- **Weekly** on a chosen day at a specific time
- **Monthly** on a day-of-month at a specific time
- **Every N minutes** for custom intervals

Each schedule targets a project (local or on a registered agent) and runs update, pull, up, restart, down, or status. Projects with a `no_updates` policy record a skipped session instead of pulling.

Projects with an enabled scheduled update are automatically excluded from the nightly background update check — no wasted Docker Hub pulls for manifests that the scheduler will check anyway.

### Backup Endpoints

Create backups from the project detail page or on a schedule. Archives are created locally first, then copied to any configured endpoint:

| Endpoint Type | Authentication |
|---------------|----------------|
| Linux / mounted path | Filesystem access |
| CIFS / NFS mount | Host-side mount |
| FTP | Username + password |
| SFTP | Password or SSH key (paste or generate from the browser) |
| S3 | Access key + secret key |

SFTP endpoints support in-browser Ed25519 key generation: click **Generate**, save the endpoint, copy the displayed public key into the remote server's `authorized_keys`. No host filesystem access needed.

### Fleet Agents & Peer Controllers

Manage Docker hosts across your network from one controller. The dashboard's **Server** selector shows **All Servers** — the local host plus every connected server — and you can filter to any single one. Add servers in Settings > Agents:

| Mode | How it works | Needs inbound port? | Manage projects? |
|------|-------------|---------------------|------------------|
| **Outbound check-in** (agent) | Agent phones home to the controller. Best behind NAT. | No | Yes, via queued commands |
| **Inbound listener** (agent) | Controller reaches out to the agent. | Yes | Yes, live |
| **Both** (agent) | Combined — works whether the host is reachable or not. | Yes | Yes, live |
| **Peer controller** | Another full Stack Manager. You add it by its URL + API key; the controller live-fetches its projects into the "All Servers" view. | Reachable over HTTPS | Yes, live |

Agents are a lightweight runtime (no database, Redis, or UI) installed with `./scripts/prepare-state.sh --agent --mode <callback\|inbound\|both> --controller https://<controller>:8993` — which auto-generates the `.env` (including `AGENT_TOKEN`), fills the controller URL (no `change-me` left behind), and prints the exact name/token to register. **Peer controllers** are two full installs that each add the other as a peer, so both dashboards see and (via the agent proxy) act on both hosts over direct HTTPS. All server-to-server traffic uses TLS 1.3.

Open any project in the "All Servers" view — including ones on a peer or agent. Peer/inbound projects are managed live. **Callback agents** can't be reached inbound, so opening their project shows a **Queued commands** panel: your up/down/pull/update/restart actions are queued and run on the agent's next check-in, with the output reported back. When a specific server is selected in the dropdown, bulk actions, Create Project, and Prune target *that* server.

### Interactive Shell

Open a live terminal inside any running container from the Shell tab. Powered by xterm.js + WebSocket + `docker exec` with a real PTY (via `creack/pty`). Full terminal support: colored prompt, tab completion, arrow keys, Ctrl-C, history, and automatic resize. Pick a container from the dropdown, click Connect, and type commands. Sessions are auth-gated and scoped to the project's containers.

### Firewall (ConfigServer csf/lfd)

Settings > Firewall provides full management of ConfigServer Firewall on the host. The server uses `nsenter` into the host's PID namespace so CSF operations (including `csf -r` which flushes all iptables) survive without killing the server container.

**Status dashboard:**
- Installed / not installed indicator with one-click Install from [Black-HOST/csf](https://github.com/Black-HOST/csf)
- Testing mode, LFD active, iptables rule count
- CSF version

**Firewall Settings panel:**
- **Testing mode** checkbox — disable after verifying your port rules
- **Docker mode** checkbox — auto-set during install on Docker hosts; ensures CSF accommodates Docker's iptables chains
- **SYN flood protection** checkbox
- **Syslog restrict** level (0–3)
- **TCP IN / TCP OUT / UDP IN / UDP OUT** — comma-separated port fields with hints explaining what each direction means
- Save + Restart csf buttons

**IP management:**
- Your detected IP shown with an **Add my IP** button
- Manual allow / deny form for any IPv4 or IPv6 address with a comment
- Allow list and deny list tables with per-entry Remove buttons
- Auto-allowlist on login — every successful dashboard login runs `csf -a` for the caller's IP

**Per-project port opening:**
- Any project has an **Open Ports (CSF)** button that adds the project's published TCP ports inbound to the host firewall (`TCP_IN`) and reloads CSF, backing up `csf.conf` first. Requires the firewall helper (`stack-manager-csf`) installed on that host.

**Config editor:**
- In-browser editor for `csf.conf`, `csf.allow`, `csf.deny`, `csf.ignore`, `csf.pignore`
- Every save creates a timestamped backup under `/var/backups/stack-manager-csf/`

**Log viewer:**
- Tail `/var/log/lfd.log` with configurable line count (up to 5000)

**Docker compatibility:**
- Install auto-sets `DOCKER = "1"` in `csf.conf`
- Writes `csfpre.sh` (saves iptables state) and `csfpost.sh` (restarts Docker after CSF reload) so `csf -r` never breaks container networking
- Installs `unzip`, `perl`, and `iptables` as prerequisites

**One-time host setup:**

```bash
sudo install -m 750 scripts/stack-manager-csf.sh /usr/local/sbin/stack-manager-csf
```

If the helper is not installed, the Firewall panel shows an amber install-command card with a Copy button instead of an error.

### Reverse Proxy (Nginx Proxy Manager)

Settings > Reverse Proxy deploys and manages Nginx Proxy Manager from the dashboard:

- **Deploy Nginx Proxy Manager** — one click stands up NPM from the built-in template and prefills the connection form with its admin URL and default login
- **Connect** with the NPM admin URL, email, and password (the auth request sends only `identity` + `secret`, which NPM's schema requires)
- **Add proxied domains** from a form — running projects appear as chips that auto-fill the forward target, plus a one-click **Stack Manager UI** target to proxy the dashboard itself
- **Add to Proxy (NPM)** on any project — one click creates a proxy host forwarding to the host + the project's first published port (domain and SSL editable in NPM after)
- **List / delete proxy hosts** from the table
- Let's Encrypt stays separate under Settings > SSL for installs that don't use the proxy; private networks and IP-only hosts can keep the built-in self-signed cert

### Two-Factor Authentication

Protect accounts with TOTP (Google Authenticator, Authy, or any compatible app):

1. Go to **Settings > Account > Set up 2FA**
2. Scan the QR code with your authenticator app
3. Save the 8 backup codes somewhere safe
4. Enter a 6-digit code to verify and enable

Once enabled, login requires a code after the password step. Admins can reset another user's 2FA from the Users tab.

### General Settings

Settings > General lets admins change `.env` values from the browser:

- **Ports** — WEB_SSL_PORT (default 8993), WEB_HTTP_PORT (default 8193)
- **Cache and refresh** — CACHE_TTL_SECONDS, METRICS_REFRESH_MINUTES, WARM_CACHE_TTL_MINUTES
- **Host URL** — the dashboard URL shown in setup output and agent commands
- **Extra Docker roots** — comma-separated additional directories to discover projects in
- **Roll API key** — generate a new API key with one click (takes effect on restart)

Port changes require a full `docker compose --env-file .env up -d` restart.

### Docker Settings

Edit the host Docker `daemon.json` from Settings > Docker Settings:

- Log driver and rotation
- DNS servers
- Default address pools
- Registry mirrors and insecure registries
- Live restore and IPv6
- Remote Docker TCP hosts (with security warnings)
- Raw JSON for advanced options

Every save creates a timestamped backup. Network-field changes show a full teardown guide.

### SSL / TLS

HTTPS works on first boot with an auto-generated self-signed certificate. For real domains:

- **Self-signed** — regenerate with custom CN and SANs from the SSL panel
- **Let's Encrypt** — set ports 80/443 and issue via HTTP-01 from the SSL panel
- **Nginx Proxy Manager** — deploy from the Stack Catalog and manage from Settings > Reverse Proxy

### Documentation

The Documentation page has two tabs:

- **Current Stacks** — auto-generated project guides for every discovered project, plus any README or docs files found in the stack directory
- **Stack Catalog** — searchable documentation for all 200+ catalog templates, with setup steps, env key references, caution notes, and upstream links

### Security & Audit

- **Image scanning** — Trivy vulnerability reports per project
- **Compose audit** — checks for privileged containers, host networking, missing healthchecks
- **Audit log** — every dashboard action logged with actor, project, IP, duration, and result
- **Per-project update policies** — auto-detect build-only repos and block accidental updates

### Project Detail

Each project has a dedicated page with tabs:

| Tab | What it shows |
|-----|--------------|
| Overview | Containers, state, update policy, image sources, compose hooks |
| Config | In-browser editor for compose.yml, .env, and other config files |
| Docs | Auto-generated project guide + any README/docs files |
| Sessions | Action history with full output logs and sticky-follow scroll |
| Sources | Image origin, registry, access status per service |
| Watch | Live startup log streaming (Up + Watch) with color-coded services |
| Logs | Docker compose logs with timestamps |
| Stats | CPU, memory, network I/O per container |
| Shell | Scoped compose commands + interactive xterm.js terminal |
| Security | Image scan + compose audit results |
| Backups | Create, restore, download, delete archives |
| Databases | Health checks and SQL dumps for supported engines |
| Inspect | Raw `docker inspect` JSON |
| Processes | `docker top` output |

The action bar also has per-project one-click buttons: **Update / Pull / Start / Restart / Stop**, **Backup**, **DB Dump**, **Open Ports (CSF)** (opens the project's published ports in the host firewall), and **Add to Proxy (NPM)** (creates an Nginx Proxy Manager host for the project). Backups warn before restore/delete, and stopping is styled as a destructive action.

---

## Architecture

```
Browser ──HTTPS──> nginx (web container, TLS termination)
                      │
                      ├── /api/* ──> Go server (port 8192, pid:host, nsenter for firewall)
                      │                 ├── MariaDB (users, jobs, settings, schedules, audit)
                      │                 ├── Redis (sessions, cache)
                      │                 ├── Docker socket (/var/run/docker.sock)
                      │                 └── WebSocket (/api/v1/projects/*/shell/exec)
                      │
                      └── /* ──> React SPA (hashed assets with immutable cache,
                                           no-cache index.html for instant deploys)
```

The same Go binary runs in two modes:

| Mode | What it does |
|------|-------------|
| **Controller** (`APP_MODE=server`) | Full stack: dashboard, API, MariaDB, Redis, agents, schedules |
| **Agent** (`APP_MODE=agent-callback/agent/agent-both`) | Lightweight: discovers local projects, reports to the controller |

---

## Installation

### Web Dashboard (recommended)

```bash
mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

`prepare-state.sh` generates cryptographically random passwords for every secret, detects `DOCKER_GID` and `SERVER_USER` from the host, and prints the login URL + credentials. Re-running it never overwrites existing secrets.

### Updating

```bash
cd ~/docker/stack-manager
git pull
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

### CLI-Only (no dashboard)

```bash
chmod +x stack-manager.sh
./stack-manager.sh --root ~/docker list
./stack-manager.sh --root ~/docker update
```

The CLI is a single Bash script with no dependencies beyond Docker Compose. It supports all lifecycle commands, bulk operations, custom update hooks, inactive markers, timeouts, dry-run mode, and logging.

---

## Environment Reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `API_KEY` | generated | API access key and bootstrap password fallback |
| `ADMIN_USERNAME` | `admin` | First admin username |
| `ADMIN_PASSWORD` | generated | First admin password (rotate after first login) |
| `DB_PASSWORD` | generated | MariaDB application password |
| `DB_ROOT_PASSWORD` | generated | MariaDB root password |
| `REDIS_PASSWORD` | generated | Redis password |
| `DOCKER_ROOT` | `~/docker` | Host directory containing managed Compose projects |
| `EXTRA_DOCKER_ROOTS` | empty | Comma-separated additional directories to scan for projects |
| `STATE_DIR` | `.stack-manager` | Persistent state (MariaDB, Redis, hooks, backups, SSL) |
| `SERVER_USER` | detected | UID:GID for the server container |
| `DOCKER_GID` | detected | Docker socket group ID |
| `WEB_SSL_PORT` | `8993` | HTTPS port (set to `443` for standard HTTPS) |
| `WEB_HTTP_PORT` | `8193` | HTTP redirect port (set to `80` for Let's Encrypt) |
| `HOST_URL` | detected | Dashboard URL shown in setup output |
| `CACHE_TTL_SECONDS` | `15` | Redis cache TTL for project state |
| `METRICS_REFRESH_MINUTES` | `15` | Background discovery and stats interval |
| `WARM_CACHE_TTL_MINUTES` | `30` | Redis TTL for background-warmed caches |

---

## Agent Installation

See [Agent Modes](docs/AGENT_MODES.md) for the full protocol reference, or use the guided setup in **Settings > Agents** which generates copy-paste commands for each mode.

```bash
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
# --mode is callback (outbound), inbound, or both. This writes the correct
# APP_MODE and auto-generates AGENT_TOKEN, AGENT_NAME, AGENT_PORT into .env,
# then prints the values to register in Settings > Agents.
./scripts/prepare-state.sh --agent --mode callback .env
# Edit .env: set DOCKER_ROOT and AGENT_CONTROLLER_URL to your controller's URL
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

Then register the agent from the controller: **Settings > Agents > Add Agent** (name, mode, and the printed `AGENT_TOKEN`). For a **peer controller** instead, run a full install on both hosts and add each as a peer of the other (URL + API key) under Settings > Agents.

---

## CLI Reference

```bash
stack-manager.sh [options] <command> [projects...]

Commands:
  list      Show discovered projects
  status    Container status per project
  check     Check for image updates
  pull      Pull latest images
  up        Start all projects
  update    Pull + recreate (respects hooks)
  restart   Restart running projects
  down      Stop all projects
  prune     Remove unused Docker resources

Options:
  --root <dir>     Project root directory
  --running        Only act on running projects
  --dry-run        Show what would run without executing
  --timeout <sec>  Timeout for pull/check operations
  --prune          Run docker system prune after operations
  --log-dir <dir>  Custom log output directory
  --no-log         Disable automatic logging
```

---

## Development

```bash
# Run tests
cd server && go test ./...
bash -n stack-manager.sh

# Build
make build           # Go + web
make build-linux     # Cross-compile for linux/amd64
make docker-up       # Full stack with docker compose

# Single test
cd server && go test ./internal/core -run TestName
```

---

## Technical Reference

Everything below is the full technical detail for operators, contributors, and anyone scripting against the CLI or API.

### Project Layout

The CLI expects projects organized under a root directory:

```
/docker/                          # Root directory (configurable with --root)
├── .stack-manager/             # Configuration directory
│   └── hooks/                    # Custom update hooks
│       └── post-update_netbox-docker.sh
├── project-a/
│   └── compose.yml
├── project-b/
│   ├── compose.yml
│   └── .inactive                 # Marker file - project is skipped
├── netbox-docker/
│   └── docker-compose.yml
└── stack-manager_20240115_143022.log  # Auto-generated log file
```

### Compose File Detection

For each subdirectory, the script looks for compose files in this order:
1. `compose.yml`
2. `compose.yaml`
3. `docker-compose.yml`
4. `docker-compose.yaml`

The first match is used.

### Update Hook Override

If `<hooks-dir>/post-update_<project>.sh` exists, `update` runs only that hook and skips the normal `docker compose pull` + `up -d`. This exists so projects like NetBox that require a specific upgrade sequence do not get broken by generic pull/up.

### Update Policies

Per-project setting stored in MariaDB, cached in Redis:

| Mode | Behavior |
|------|----------|
| `auto` (default) | Build-only GitHub/GitLab projects (no registry image) are auto-treated as `no_updates`; others allow updates |
| `allow` | Always run update actions |
| `no_updates` | Skip update actions and record a skipped action session with the configured reason |

Scheduled updates and bulk actions respect this policy.

### Project Deletion Guardrails

Directory deletion (CLI and API) enforces:

1. Project must be marked inactive first.
2. Exact project-name confirmation required.
3. Target must be a discovered project.
4. Refuse to delete the configured `DOCKER_ROOT` or any path outside it.

By default deletion runs `docker compose down` before removing the project directory.

### Backup Endpoint Details

Project backups are always created locally under `BACKUP_DIR` first, then copied/uploaded to the selected endpoint. Endpoint types (`Linux path`, `Mounted path`, `CIFS mount`, `NFS mount`, `FTP`, `SFTP`, `S3`) are managed in Settings, stored in MariaDB, and secrets are never returned to the browser after save. FTP/SFTP/S3 use `rclone`; local/mounted just copy under `/backup-targets`.

If a non-root server user cannot read project data (e.g. Postgres/MariaDB/Redis bind mounts), backup falls back to a short-lived root helper container through the Docker socket.

### Background Cache And Metrics

Stack Manager warms project discovery, update-policy metadata, image-source metadata, and container stats in the background every `METRICS_REFRESH_MINUTES`. The projects:list cache TTL is capped at 30 minutes regardless of the metrics interval so a very large METRICS_REFRESH_MINUTES cannot freeze the dashboard STATE column. Dashboard reads use Redis-cached summaries so normal page loads do not wait for Docker inspection commands.

Metrics stored in MariaDB for historical graphing:

| Metric | Source |
|--------|--------|
| CPU and memory | `docker stats --no-stream` sampled in the background |
| Inbound/outbound traffic | Docker network RX/TX counters |
| Backup count/bytes | Backup skill create events |
| Restore count/bytes | Backup skill restore events |
| Upload bytes | Backup endpoint copy/upload events |

### Root-Sensitive Projects

GitLab, PMM, and similar stacks may run containers as root internally or require root-owned data. Stack Manager does not need to run as host root for the containers themselves; Docker handles container users. Stack Manager needs filesystem access to the project directory and compose files under `DOCKER_ROOT`.

Recommended model for mixed hosts:

1. Run Stack Manager as the host service user (`SERVER_USER=1000:1000`).
2. Keep compose files and `.env` readable by that UID/GID.
3. Store application data in Docker named volumes where possible.
4. For root-only project directories, run a separate root-capable Stack Manager agent with `SERVER_USER=0:0` and register it from the main server.

### Persistent State

| Path | Purpose |
|------|---------|
| `mariadb/` | MariaDB data for users, jobs, project settings, agents, schedules |
| `redis/` | Redis append-only data for sessions/cache |
| `hooks/` | Update hooks used by the API server |
| `backups/` | Project backups and database dumps |
| `backup-targets/` | Default host-backed mount for UI-configured backup destinations |
| `docker-config/` | Docker registry credentials from dashboard registry login |
| `ssl/` | TLS certificates (self-signed or Let's Encrypt) |

### Inactive Project Management

```bash
# Mark a project inactive (creates .inactive file)
touch /docker/project-name/.inactive

# Re-activate
rm /docker/project-name/.inactive

# List only active projects
stack-manager.sh --root /docker list
```

Inactive projects are excluded from `status`, `check`, `pull`, `update`, `up`, `restart`, `down`, and `prune` unless targeted by name.

### Custom Update Hooks

```bash
# Create a hook for a project
cat > ~/.stack-manager/hooks/post-update_netbox-docker.sh << 'EOF'
#!/bin/bash
cd "$PROJECT_DIR"
git pull
docker compose pull
docker compose up -d
EOF
chmod +x ~/.stack-manager/hooks/post-update_netbox-docker.sh
```

When `update` runs for `netbox-docker`, only this hook executes. Normal pull/up is skipped.

### Logging

All operations are logged to timestamped files:

```
/docker/stack-manager_20240115_143022.log
```

Override the log directory with `--log-dir` or disable with `--no-log`.

### Signal Handling

Ctrl-C during a batch run interrupts the current project and prints a summary of completed projects. No projects after the current one are started.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All operations completed |
| 1 | One or more operations failed |
| 2 | Invalid arguments or configuration |
| 130 | Interrupted by Ctrl-C |

### GitLab Pipeline

The GitLab pipeline treats docker02 as the dev environment:

- `deploy:docker02` runs automatically on the default branch after validation, tests, builds, and security scans pass.
- The deploy job preserves existing `.env` secrets or generates secure first-run values.
- `smoke:docker02` runs automatically after deploy.
- `smoke:stack-template` and `smoke:stack-templates:all` are manual test-server jobs. Set `STACK_TEMPLATE_TEST_HOST` to the test server IP. Use `--skip` to exclude specific templates.
- `push:github` is an optional manual job that mirrors tested `main` to GitHub.

---

## License

See [LICENSE](LICENSE) for details.
