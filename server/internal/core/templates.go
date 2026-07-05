package core

import (
	"fmt"
	"sort"
	"strings"
)

type StackTemplate struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	Source         string   `json:"source"`
	Image          string   `json:"image,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	ComposeContent string   `json:"compose_content"`
	EnvContent     string   `json:"env_content,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

func BuiltinStackTemplates() []StackTemplate {
	templates := []StackTemplate{
		{
			ID:          "wordpress-mariadb",
			Name:        "WordPress + MariaDB",
			Description: "Blog/CMS stack with WordPress and MariaDB.",
			Category:    "cms",
			Source:      "docker-docs-portainer-style",
			Image:       "wordpress:latest",
			Tags:        []string{"cms", "database", "website"},
			ComposeContent: `services:
  wordpress:
    image: wordpress:latest
    restart: unless-stopped
    ports:
      - "${WORDPRESS_PORT:-8080}:80"
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: ${WORDPRESS_DB_USER:-wordpress}
      WORDPRESS_DB_PASSWORD: ${WORDPRESS_DB_PASSWORD:-change-me}
      WORDPRESS_DB_NAME: ${WORDPRESS_DB_NAME:-wordpress}
    volumes:
      - wordpress-data:/var/www/html
    depends_on:
      - db
  db:
    image: mariadb:11.4
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: ${WORDPRESS_DB_NAME:-wordpress}
      MARIADB_USER: ${WORDPRESS_DB_USER:-wordpress}
      MARIADB_PASSWORD: ${WORDPRESS_DB_PASSWORD:-change-me}
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:-change-me-root}
    volumes:
      - db-data:/var/lib/mysql
volumes:
  wordpress-data:
  db-data:
`,
			EnvContent: `WORDPRESS_PORT=8080
WORDPRESS_DB_NAME=wordpress
WORDPRESS_DB_USER=wordpress
WORDPRESS_DB_PASSWORD=change-me
MARIADB_ROOT_PASSWORD=change-me-root
`,
			Notes: "Change the database passwords before starting this stack.",
		},
		{
			ID:          "nginx-static",
			Name:        "Nginx Static Site",
			Description: "Small static web server with a bind-mounted html directory.",
			Category:    "web",
			Source:      "kitematic-style",
			Image:       "nginx:stable-alpine",
			Tags:        []string{"web", "static", "nginx"},
			ComposeContent: `services:
  web:
    image: nginx:stable-alpine
    restart: unless-stopped
    ports:
      - "${WEB_PORT:-8080}:80"
    volumes:
      - ./html:/usr/share/nginx/html:ro
`,
			EnvContent: "WEB_PORT=8080\n",
			Notes:      "Create an html directory beside compose.yml before starting, or edit the bind mount.",
		},
		{
			ID:          "postgres",
			Name:        "PostgreSQL",
			Description: "PostgreSQL database with persistent volume.",
			Category:    "database",
			Source:      "kitematic-style",
			Image:       "postgres:16-alpine",
			Tags:        []string{"database", "postgres"},
			ComposeContent: `services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-app}
      POSTGRES_USER: ${POSTGRES_USER:-app}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - postgres-data:/var/lib/postgresql/data
volumes:
  postgres-data:
`,
			EnvContent: `POSTGRES_PORT=5432
POSTGRES_DB=app
POSTGRES_USER=app
POSTGRES_PASSWORD=change-me
`,
		},
		{
			ID:          "redis",
			Name:        "Redis",
			Description: "Redis cache with append-only persistence and password auth.",
			Category:    "database",
			Source:      "kitematic-style",
			Image:       "redis:7.4-alpine",
			Tags:        []string{"cache", "redis"},
			ComposeContent: `services:
  redis:
    image: redis:7.4-alpine
    restart: unless-stopped
    command: ["redis-server", "--appendonly", "yes", "--requirepass", "${REDIS_PASSWORD:-change-me}"]
    ports:
      - "${REDIS_PORT:-6379}:6379"
    volumes:
      - redis-data:/data
volumes:
  redis-data:
`,
			EnvContent: `REDIS_PORT=6379
REDIS_PASSWORD=change-me
`,
		},
		{
			ID:          "gitea",
			Name:        "Gitea",
			Description: "Lightweight Git service with SSH and web ports.",
			Category:    "devtools",
			Source:      "portainer-style",
			Image:       "gitea/gitea:latest",
			Tags:        []string{"git", "devtools"},
			ComposeContent: `services:
  gitea:
    image: gitea/gitea:latest
    restart: unless-stopped
    environment:
      USER_UID: ${USER_UID:-1000}
      USER_GID: ${USER_GID:-1000}
    ports:
      - "${GITEA_WEB_PORT:-3000}:3000"
      - "${GITEA_SSH_PORT:-2222}:22"
    volumes:
      - gitea-data:/data
volumes:
  gitea-data:
`,
			EnvContent: `USER_UID=1000
USER_GID=1000
GITEA_WEB_PORT=3000
GITEA_SSH_PORT=2222
`,
		},
		{
			ID:          "uptime-kuma",
			Name:        "Uptime Kuma",
			Description: "Self-hosted uptime monitor.",
			Category:    "monitoring",
			Source:      "portainer-style",
			Image:       "louislam/uptime-kuma:1",
			Tags:        []string{"monitoring", "status"},
			ComposeContent: `services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    restart: unless-stopped
    ports:
      - "${UPTIME_KUMA_PORT:-3001}:3001"
    volumes:
      - uptime-kuma-data:/app/data
volumes:
  uptime-kuma-data:
`,
			EnvContent: "UPTIME_KUMA_PORT=3001\n",
		},
		{
			ID:          "portainer-agent",
			Name:        "Portainer Agent",
			Description: "Portainer agent for hosts that also need Portainer compatibility.",
			Category:    "management",
			Source:      "portainer-style",
			Image:       "portainer/agent:latest",
			Tags:        []string{"management", "agent"},
			ComposeContent: `services:
  portainer-agent:
    image: portainer/agent:latest
    restart: unless-stopped
    ports:
      - "${PORTAINER_AGENT_PORT:-9001}:9001"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /var/lib/docker/volumes:/var/lib/docker/volumes
`,
			EnvContent: "PORTAINER_AGENT_PORT=9001\n",
		},
		{
			ID:          "prometheus-grafana",
			Name:        "Prometheus + Grafana",
			Description: "Monitoring starter stack with Prometheus and Grafana.",
			Category:    "monitoring",
			Source:      "rancher-style",
			Image:       "grafana/grafana-oss:latest",
			Tags:        []string{"monitoring", "metrics", "grafana"},
			ComposeContent: `services:
  prometheus:
    image: prom/prometheus:latest
    restart: unless-stopped
    ports:
      - "${PROMETHEUS_PORT:-9090}:9090"
    volumes:
      - prometheus-data:/prometheus
  grafana:
    image: grafana/grafana-oss:latest
    restart: unless-stopped
    ports:
      - "${GRAFANA_PORT:-3000}:3000"
    volumes:
      - grafana-data:/var/lib/grafana
volumes:
  prometheus-data:
  grafana-data:
`,
			EnvContent: `PROMETHEUS_PORT=9090
GRAFANA_PORT=3000
`,
			Notes: "Add a prometheus.yml bind mount before production use.",
		},
		{
			ID:          "traefik",
			Name:        "Traefik",
			Description: "Docker-aware reverse proxy with dashboard.",
			Category:    "proxy",
			Source:      "portainer-style",
			Image:       "traefik:v3.1",
			Tags:        []string{"proxy", "ingress", "tls"},
			ComposeContent: `services:
  traefik:
    image: traefik:v3.1
    restart: unless-stopped
    command:
      - --api.dashboard=true
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
    ports:
      - "${TRAEFIK_HTTP_PORT:-80}:80"
      - "${TRAEFIK_DASHBOARD_PORT:-8080}:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
`,
			EnvContent: "TRAEFIK_HTTP_PORT=80\nTRAEFIK_DASHBOARD_PORT=8080\n",
		},
		{
			ID:          "caddy",
			Name:        "Caddy",
			Description: "Simple HTTPS-ready web reverse proxy.",
			Category:    "proxy",
			Source:      "kitematic-style",
			Image:       "caddy:2-alpine",
			Tags:        []string{"proxy", "web", "tls"},
			ComposeContent: `services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "${HTTP_PORT:-80}:80"
      - "${HTTPS_PORT:-443}:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
volumes:
  caddy-data:
  caddy-config:
`,
			EnvContent: "HTTP_PORT=80\nHTTPS_PORT=443\n",
			Notes:      "Create a Caddyfile beside compose.yml before starting.",
		},
		{
			ID:          "adminer",
			Name:        "Adminer",
			Description: "Small database administration UI.",
			Category:    "database",
			Source:      "kitematic-style",
			Image:       "adminer:latest",
			Tags:        []string{"database", "admin"},
			ComposeContent: `services:
  adminer:
    image: adminer:latest
    restart: unless-stopped
    ports:
      - "${ADMINER_PORT:-8080}:8080"
`,
			EnvContent: "ADMINER_PORT=8080\n",
		},
		{
			ID:          "mysql",
			Name:        "MySQL",
			Description: "MySQL database with persistent storage.",
			Category:    "database",
			Source:      "kitematic-style",
			Image:       "mysql:8.4",
			Tags:        []string{"database", "mysql"},
			ComposeContent: `services:
  mysql:
    image: mysql:8.4
    restart: unless-stopped
    ports:
      - "${MYSQL_PORT:-3306}:3306"
    environment:
      MYSQL_DATABASE: ${MYSQL_DATABASE:-app}
      MYSQL_USER: ${MYSQL_USER:-app}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:-change-me}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:-change-me-root}
    volumes:
      - mysql-data:/var/lib/mysql
volumes:
  mysql-data:
`,
			EnvContent: "MYSQL_PORT=3306\nMYSQL_DATABASE=app\nMYSQL_USER=app\nMYSQL_PASSWORD=change-me\nMYSQL_ROOT_PASSWORD=change-me-root\n",
		},
		{
			ID:          "mongo",
			Name:        "MongoDB",
			Description: "MongoDB document database with root credentials.",
			Category:    "database",
			Source:      "kitematic-style",
			Image:       "mongo:7",
			Tags:        []string{"database", "mongodb"},
			ComposeContent: `services:
  mongo:
    image: mongo:7
    restart: unless-stopped
    ports:
      - "${MONGO_PORT:-27017}:27017"
    environment:
      MONGO_INITDB_ROOT_USERNAME: ${MONGO_ROOT_USER:-admin}
      MONGO_INITDB_ROOT_PASSWORD: ${MONGO_ROOT_PASSWORD:-change-me}
    volumes:
      - mongo-data:/data/db
volumes:
  mongo-data:
`,
			EnvContent: "MONGO_PORT=27017\nMONGO_ROOT_USER=admin\nMONGO_ROOT_PASSWORD=change-me\n",
		},
		{
			ID:          "rabbitmq",
			Name:        "RabbitMQ",
			Description: "Message broker with management console.",
			Category:    "queue",
			Source:      "kitematic-style",
			Image:       "rabbitmq:3-management-alpine",
			Tags:        []string{"queue", "broker", "management"},
			ComposeContent: `services:
  rabbitmq:
    image: rabbitmq:3-management-alpine
    restart: unless-stopped
    ports:
      - "${RABBITMQ_PORT:-5672}:5672"
      - "${RABBITMQ_UI_PORT:-15672}:15672"
    environment:
      RABBITMQ_DEFAULT_USER: ${RABBITMQ_USER:-admin}
      RABBITMQ_DEFAULT_PASS: ${RABBITMQ_PASSWORD:-change-me}
    volumes:
      - rabbitmq-data:/var/lib/rabbitmq
volumes:
  rabbitmq-data:
`,
			EnvContent: "RABBITMQ_PORT=5672\nRABBITMQ_UI_PORT=15672\nRABBITMQ_USER=admin\nRABBITMQ_PASSWORD=change-me\n",
		},
		{
			ID:          "nextcloud",
			Name:        "Nextcloud",
			Description: "File sync and sharing with MariaDB.",
			Category:    "files",
			Source:      "portainer-style",
			Image:       "nextcloud:latest",
			Tags:        []string{"files", "sync", "mariadb"},
			ComposeContent: `services:
  nextcloud:
    image: nextcloud:latest
    restart: unless-stopped
    ports:
      - "${NEXTCLOUD_PORT:-8080}:80"
    environment:
      MYSQL_HOST: db
      MYSQL_DATABASE: ${MYSQL_DATABASE:-nextcloud}
      MYSQL_USER: ${MYSQL_USER:-nextcloud}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:-change-me}
    volumes:
      - nextcloud-data:/var/www/html
    depends_on:
      - db
  db:
    image: mariadb:11.4
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: ${MYSQL_DATABASE:-nextcloud}
      MARIADB_USER: ${MYSQL_USER:-nextcloud}
      MARIADB_PASSWORD: ${MYSQL_PASSWORD:-change-me}
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:-change-me-root}
    volumes:
      - db-data:/var/lib/mysql
volumes:
  nextcloud-data:
  db-data:
`,
			EnvContent: "NEXTCLOUD_PORT=8080\nMYSQL_DATABASE=nextcloud\nMYSQL_USER=nextcloud\nMYSQL_PASSWORD=change-me\nMARIADB_ROOT_PASSWORD=change-me-root\n",
		},
		{
			ID:          "vaultwarden",
			Name:        "Vaultwarden",
			Description: "Lightweight Bitwarden-compatible password vault.",
			Category:    "security",
			Source:      "portainer-style",
			Image:       "vaultwarden/server:latest",
			Tags:        []string{"passwords", "vault", "security"},
			ComposeContent: `services:
  vaultwarden:
    image: vaultwarden/server:latest
    restart: unless-stopped
    ports:
      - "${VAULTWARDEN_PORT:-8080}:80"
    environment:
      SIGNUPS_ALLOWED: "${SIGNUPS_ALLOWED:-false}"
    volumes:
      - vaultwarden-data:/data
volumes:
  vaultwarden-data:
`,
			EnvContent: "VAULTWARDEN_PORT=8080\nSIGNUPS_ALLOWED=false\n",
		},
		{
			ID:          "bookstack",
			Name:        "BookStack",
			Description: "Documentation/wiki platform with MariaDB.",
			Category:    "docs",
			Source:      "portainer-style",
			Image:       "lscr.io/linuxserver/bookstack:latest",
			Tags:        []string{"docs", "wiki", "knowledge-base"},
			ComposeContent: `services:
  bookstack:
    image: lscr.io/linuxserver/bookstack:latest
    restart: unless-stopped
    ports:
      - "${BOOKSTACK_PORT:-6875}:80"
    environment:
      APP_URL: ${APP_URL:-http://localhost:6875}
      DB_HOST: db
      DB_DATABASE: ${DB_DATABASE:-bookstack}
      DB_USERNAME: ${DB_USERNAME:-bookstack}
      DB_PASSWORD: ${DB_PASSWORD:-change-me}
    volumes:
      - bookstack-data:/config
    depends_on:
      - db
  db:
    image: mariadb:11.4
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: ${DB_DATABASE:-bookstack}
      MARIADB_USER: ${DB_USERNAME:-bookstack}
      MARIADB_PASSWORD: ${DB_PASSWORD:-change-me}
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:-change-me-root}
    volumes:
      - db-data:/var/lib/mysql
volumes:
  bookstack-data:
  db-data:
`,
			EnvContent: "BOOKSTACK_PORT=6875\nAPP_URL=http://localhost:6875\nDB_DATABASE=bookstack\nDB_USERNAME=bookstack\nDB_PASSWORD=change-me\nMARIADB_ROOT_PASSWORD=change-me-root\n",
		},
		{
			ID:          "n8n",
			Name:        "n8n",
			Description: "Workflow automation tool.",
			Category:    "automation",
			Source:      "portainer-style",
			Image:       "n8nio/n8n:latest",
			Tags:        []string{"automation", "workflow"},
			ComposeContent: `services:
  n8n:
    image: n8nio/n8n:latest
    restart: unless-stopped
    ports:
      - "${N8N_PORT:-5678}:5678"
    environment:
      N8N_HOST: ${N8N_HOST:-localhost}
      N8N_PROTOCOL: ${N8N_PROTOCOL:-http}
      WEBHOOK_URL: ${WEBHOOK_URL:-http://localhost:5678/}
    volumes:
      - n8n-data:/home/node/.n8n
volumes:
  n8n-data:
`,
			EnvContent: "N8N_PORT=5678\nN8N_HOST=localhost\nN8N_PROTOCOL=http\nWEBHOOK_URL=http://localhost:5678/\n",
		},
		{
			ID:          "home-assistant",
			Name:        "Home Assistant",
			Description: "Home automation server.",
			Category:    "automation",
			Source:      "kitematic-style",
			Image:       "ghcr.io/home-assistant/home-assistant:stable",
			Tags:        []string{"automation", "home"},
			ComposeContent: `services:
  home-assistant:
    image: ghcr.io/home-assistant/home-assistant:stable
    restart: unless-stopped
    network_mode: host
    volumes:
      - home-assistant-config:/config
volumes:
  home-assistant-config:
`,
			Notes: "This template uses host networking because many Home Assistant integrations expect it.",
		},
		{
			ID:          "jellyfin",
			Name:        "Jellyfin",
			Description: "Self-hosted media server.",
			Category:    "media",
			Source:      "kitematic-style",
			Image:       "jellyfin/jellyfin:latest",
			Tags:        []string{"media", "video"},
			ComposeContent: `services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    restart: unless-stopped
    ports:
      - "${JELLYFIN_PORT:-8096}:8096"
    volumes:
      - jellyfin-config:/config
      - jellyfin-cache:/cache
      - ./media:/media:ro
volumes:
  jellyfin-config:
  jellyfin-cache:
`,
			EnvContent: "JELLYFIN_PORT=8096\n",
			Notes:      "Create a media directory or edit the bind mount before starting.",
		},
		{
			ID:          "code-server",
			Name:        "code-server",
			Description: "Browser-based VS Code server.",
			Category:    "devtools",
			Source:      "kitematic-style",
			Image:       "lscr.io/linuxserver/code-server:latest",
			Tags:        []string{"devtools", "ide"},
			ComposeContent: `services:
  code-server:
    image: lscr.io/linuxserver/code-server:latest
    restart: unless-stopped
    ports:
      - "${CODE_SERVER_PORT:-8443}:8443"
    environment:
      PASSWORD: ${CODE_SERVER_PASSWORD:-change-me}
    volumes:
      - code-server-config:/config
      - ./workspace:/config/workspace
volumes:
  code-server-config:
`,
			EnvContent: "CODE_SERVER_PORT=8443\nCODE_SERVER_PASSWORD=change-me\n",
		},
		{
			ID:          "dozzle",
			Name:        "Dozzle",
			Description: "Lightweight Docker log viewer.",
			Category:    "management",
			Source:      "portainer-style",
			Image:       "amir20/dozzle:latest",
			Tags:        []string{"logs", "management"},
			ComposeContent: `services:
  dozzle:
    image: amir20/dozzle:latest
    restart: unless-stopped
    ports:
      - "${DOZZLE_PORT:-8080}:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
`,
			EnvContent: "DOZZLE_PORT=8080\n",
		},
		{
			ID:          "changedetection",
			Name:        "changedetection.io",
			Description: "Website change monitoring.",
			Category:    "monitoring",
			Source:      "portainer-style",
			Image:       "dgtlmoon/changedetection.io:latest",
			Tags:        []string{"monitoring", "web"},
			ComposeContent: `services:
  changedetection:
    image: dgtlmoon/changedetection.io:latest
    restart: unless-stopped
    ports:
      - "${CHANGEDETECTION_PORT:-5000}:5000"
    volumes:
      - changedetection-data:/datastore
volumes:
  changedetection-data:
`,
			EnvContent: "CHANGEDETECTION_PORT=5000\n",
		},
		{
			ID:          "homepage",
			Name:        "Homepage",
			Description: "Clean self-hosted dashboard for services and widgets.",
			Category:    "management",
			Source:      "portainer-style",
			Image:       "ghcr.io/gethomepage/homepage:latest",
			Tags:        []string{"dashboard", "homepage", "management"},
			ComposeContent: `services:
  homepage:
    image: ghcr.io/gethomepage/homepage:latest
    restart: unless-stopped
    ports:
      - "${HOMEPAGE_PORT:-3000}:3000"
    volumes:
      - homepage-config:/app/config
      - /var/run/docker.sock:/var/run/docker.sock:ro
volumes:
  homepage-config:
`,
			EnvContent: "HOMEPAGE_PORT=3000\n",
			Notes:      "The Docker socket mount lets Homepage discover container status. Remove it if you only want static links.",
		},
		{
			ID:          "it-tools",
			Name:        "IT Tools",
			Description: "Browser toolbox for encoding, crypto, network, and dev utilities.",
			Category:    "devtools",
			Source:      "portainer-style",
			Image:       "corentinth/it-tools:latest",
			Tags:        []string{"devtools", "utilities", "web"},
			ComposeContent: `services:
  it-tools:
    image: corentinth/it-tools:latest
    restart: unless-stopped
    ports:
      - "${IT_TOOLS_PORT:-8080}:80"
`,
			EnvContent: "IT_TOOLS_PORT=8080\n",
		},
		{
			ID:          "mealie",
			Name:        "Mealie",
			Description: "Recipe manager with meal planning and shopping lists.",
			Category:    "automation",
			Source:      "portainer-style",
			Image:       "ghcr.io/mealie-recipes/mealie:latest",
			Tags:        []string{"recipes", "meal-planning", "family"},
			ComposeContent: `services:
  mealie:
    image: ghcr.io/mealie-recipes/mealie:latest
    restart: unless-stopped
    ports:
      - "${MEALIE_PORT:-9925}:9000"
    environment:
      ALLOW_SIGNUP: "${ALLOW_SIGNUP:-false}"
      TZ: ${TZ:-UTC}
    volumes:
      - mealie-data:/app/data
volumes:
  mealie-data:
`,
			EnvContent: "MEALIE_PORT=9925\nALLOW_SIGNUP=false\nTZ=UTC\n",
		},
		{
			ID:          "minio",
			Name:        "MinIO",
			Description: "S3-compatible object storage with web console.",
			Category:    "files",
			Source:      "portainer-style",
			Image:       "quay.io/minio/minio:latest",
			Tags:        []string{"storage", "s3", "object-storage"},
			ComposeContent: `services:
  minio:
    image: quay.io/minio/minio:latest
    restart: unless-stopped
    command: server /data --console-address ":9001"
    ports:
      - "${MINIO_API_PORT:-9000}:9000"
      - "${MINIO_CONSOLE_PORT:-9001}:9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-admin}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD:-change-me-minio}
    volumes:
      - minio-data:/data
volumes:
  minio-data:
`,
			EnvContent: "MINIO_API_PORT=9000\nMINIO_CONSOLE_PORT=9001\nMINIO_ROOT_USER=admin\nMINIO_ROOT_PASSWORD=change-me-minio\n",
			Notes:      "Change MINIO_ROOT_PASSWORD before exposing the console.",
		},
		{
			ID:          "nocodb",
			Name:        "NocoDB",
			Description: "Spreadsheet-style database UI and no-code app builder.",
			Category:    "database",
			Source:      "portainer-style",
			Image:       "nocodb/nocodb:latest",
			Tags:        []string{"database", "no-code", "spreadsheet"},
			ComposeContent: `services:
  nocodb:
    image: nocodb/nocodb:latest
    restart: unless-stopped
    ports:
      - "${NOCODB_PORT:-8080}:8080"
    volumes:
      - nocodb-data:/usr/app/data
volumes:
  nocodb-data:
`,
			EnvContent: "NOCODB_PORT=8080\n",
		},
		{
			ID:          "ollama-open-webui",
			Name:        "Ollama + Open WebUI",
			Description: "Local AI model runner with a web chat interface.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "ghcr.io/open-webui/open-webui:main",
			Tags:        []string{"ai", "llm", "chat"},
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    restart: unless-stopped
    volumes:
      - ollama-data:/root/.ollama
  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    restart: unless-stopped
    ports:
      - "${OPEN_WEBUI_PORT:-3000}:8080"
    environment:
      OLLAMA_BASE_URL: http://ollama:11434
    volumes:
      - open-webui-data:/app/backend/data
    depends_on:
      - ollama
volumes:
  ollama-data:
  open-webui-data:
`,
			EnvContent: "OPEN_WEBUI_PORT=3000\n",
			Notes:      "CPU-only model serving can be slow. Add GPU runtime options manually if the host supports them.",
		},
		{
			ID:          "paperless-ngx",
			Name:        "Paperless-ngx",
			Description: "Document intake, OCR, tagging, and archive search.",
			Category:    "docs",
			Source:      "portainer-style",
			Image:       "ghcr.io/paperless-ngx/paperless-ngx:latest",
			Tags:        []string{"documents", "ocr", "archive"},
			ComposeContent: `services:
  broker:
    image: redis:7.4-alpine
    restart: unless-stopped
    volumes:
      - redis-data:/data
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${PAPERLESS_DBNAME:-paperless}
      POSTGRES_USER: ${PAPERLESS_DBUSER:-paperless}
      POSTGRES_PASSWORD: ${PAPERLESS_DBPASS:-change-me}
    volumes:
      - postgres-data:/var/lib/postgresql/data
  webserver:
    image: ghcr.io/paperless-ngx/paperless-ngx:latest
    restart: unless-stopped
    depends_on:
      - db
      - broker
    ports:
      - "${PAPERLESS_PORT:-8000}:8000"
    environment:
      PAPERLESS_REDIS: redis://broker:6379
      PAPERLESS_DBHOST: db
      PAPERLESS_DBNAME: ${PAPERLESS_DBNAME:-paperless}
      PAPERLESS_DBUSER: ${PAPERLESS_DBUSER:-paperless}
      PAPERLESS_DBPASS: ${PAPERLESS_DBPASS:-change-me}
      PAPERLESS_SECRET_KEY: ${PAPERLESS_SECRET_KEY:-change-me-secret}
      PAPERLESS_URL: ${PAPERLESS_URL:-http://localhost:8000}
    volumes:
      - paperless-data:/usr/src/paperless/data
      - paperless-media:/usr/src/paperless/media
      - ./consume:/usr/src/paperless/consume
volumes:
  redis-data:
  postgres-data:
  paperless-data:
  paperless-media:
`,
			EnvContent: "PAPERLESS_PORT=8000\nPAPERLESS_URL=http://localhost:8000\nPAPERLESS_DBNAME=paperless\nPAPERLESS_DBUSER=paperless\nPAPERLESS_DBPASS=change-me\nPAPERLESS_SECRET_KEY=change-me-secret\n",
			Notes:      "Create a consume directory beside compose.yml or edit the bind mount before starting.",
		},
		{
			ID:          "stirling-pdf",
			Name:        "Stirling PDF",
			Description: "Self-hosted PDF merge, split, convert, and repair tools.",
			Category:    "docs",
			Source:      "portainer-style",
			Image:       "frooodle/s-pdf:latest",
			Tags:        []string{"pdf", "documents", "tools"},
			ComposeContent: `services:
  stirling-pdf:
    image: frooodle/s-pdf:latest
    restart: unless-stopped
    ports:
      - "${STIRLING_PDF_PORT:-8080}:8080"
    volumes:
      - stirling-training-data:/usr/share/tessdata
      - stirling-config:/configs
volumes:
  stirling-training-data:
  stirling-config:
`,
			EnvContent: "STIRLING_PDF_PORT=8080\n",
		},
		{
			ID:          "syncthing",
			Name:        "Syncthing",
			Description: "Peer-to-peer file synchronization.",
			Category:    "files",
			Source:      "portainer-style",
			Image:       "syncthing/syncthing:latest",
			Tags:        []string{"sync", "files", "p2p"},
			ComposeContent: `services:
  syncthing:
    image: syncthing/syncthing:latest
    restart: unless-stopped
    ports:
      - "${SYNCTHING_UI_PORT:-8384}:8384"
      - "${SYNCTHING_TCP_PORT:-22000}:22000/tcp"
      - "${SYNCTHING_UDP_PORT:-22000}:22000/udp"
      - "${SYNCTHING_DISCOVERY_PORT:-21027}:21027/udp"
    volumes:
      - syncthing-config:/var/syncthing
      - ./sync:/data
volumes:
  syncthing-config:
`,
			EnvContent: "SYNCTHING_UI_PORT=8384\nSYNCTHING_TCP_PORT=22000\nSYNCTHING_UDP_PORT=22000\nSYNCTHING_DISCOVERY_PORT=21027\n",
			Notes:      "Create a sync directory beside compose.yml or edit the bind mount before starting.",
		},
		{
			ID:          "ollama",
			Name:        "Ollama",
			Description: "Local LLM model runner API.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "ollama/ollama:latest",
			Tags:        []string{"ai", "llm", "models"},
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    restart: unless-stopped
    ports:
      - "${OLLAMA_PORT:-11434}:11434"
    volumes:
      - ollama-data:/root/.ollama
volumes:
  ollama-data:
`,
			EnvContent: "OLLAMA_PORT=11434\n",
			Notes:      "Pull models after startup with docker compose exec ollama ollama pull llama3.1 or another model.",
		},
		{
			ID:          "anythingllm",
			Name:        "AnythingLLM",
			Description: "Private AI workspace with document chat and agents.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "mintplexlabs/anythingllm:latest",
			Tags:        []string{"ai", "rag", "documents"},
			ComposeContent: `services:
  anythingllm:
    image: mintplexlabs/anythingllm:latest
    restart: unless-stopped
    ports:
      - "${ANYTHINGLLM_PORT:-3001}:3001"
    cap_add:
      - SYS_ADMIN
    environment:
      STORAGE_DIR: /app/server/storage
    volumes:
      - anythingllm-storage:/app/server/storage
volumes:
  anythingllm-storage:
`,
			EnvContent: "ANYTHINGLLM_PORT=3001\n",
			Notes:      "Connect it to Ollama, OpenAI-compatible APIs, or other providers from the AnythingLLM setup screen.",
		},
		{
			ID:          "vllm-openai",
			Name:        "vLLM OpenAI Server",
			Description: "High-throughput OpenAI-compatible LLM inference server.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "vllm/vllm-openai:latest",
			Tags:        []string{"ai", "llm", "openai-compatible", "inference"},
			ComposeContent: `services:
  vllm:
    image: vllm/vllm-openai:latest
    restart: unless-stopped
    ipc: host
    ports:
      - "${VLLM_PORT:-8000}:8000"
    command:
      - --host
      - 0.0.0.0
      - --port
      - "8000"
      - --model
      - ${VLLM_MODEL:-facebook/opt-125m}
    environment:
      HUGGING_FACE_HUB_TOKEN: ${HUGGING_FACE_HUB_TOKEN:-}
    volumes:
      - vllm-cache:/root/.cache/huggingface
volumes:
  vllm-cache:
`,
			EnvContent: "VLLM_PORT=8000\nVLLM_MODEL=facebook/opt-125m\nHUGGING_FACE_HUB_TOKEN=\n",
			Notes:      "vLLM is intended for GPU inference. Add the host-specific GPU runtime/device settings before using large models.",
		},
		{
			ID:          "text-generation-inference",
			Name:        "Text Generation Inference",
			Description: "Hugging Face LLM inference server.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "ghcr.io/huggingface/text-generation-inference:latest",
			Tags:        []string{"ai", "llm", "huggingface", "inference"},
			ComposeContent: `services:
  tgi:
    image: ghcr.io/huggingface/text-generation-inference:latest
    restart: unless-stopped
    shm_size: 1g
    ports:
      - "${TGI_PORT:-8080}:80"
    command:
      - --model-id
      - ${TGI_MODEL_ID:-HuggingFaceH4/zephyr-7b-beta}
    environment:
      HUGGING_FACE_HUB_TOKEN: ${HUGGING_FACE_HUB_TOKEN:-}
    volumes:
      - tgi-cache:/data
volumes:
  tgi-cache:
`,
			EnvContent: "TGI_PORT=8080\nTGI_MODEL_ID=HuggingFaceH4/zephyr-7b-beta\nHUGGING_FACE_HUB_TOKEN=\n",
			Notes:      "Most useful TGI deployments need GPU runtime settings and enough VRAM for the selected model.",
		},
		{
			ID:          "litellm-proxy",
			Name:        "LiteLLM Proxy",
			Description: "OpenAI-compatible proxy for many LLM providers.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "ghcr.io/berriai/litellm:main-latest",
			Tags:        []string{"ai", "proxy", "openai-compatible", "gateway"},
			ComposeContent: `services:
  litellm:
    image: ghcr.io/berriai/litellm:main-latest
    restart: unless-stopped
    ports:
      - "${LITELLM_PORT:-4000}:4000"
    environment:
      LITELLM_MASTER_KEY: ${LITELLM_MASTER_KEY:-change-me}
      DATABASE_URL: postgresql://${POSTGRES_USER:-litellm}:${POSTGRES_PASSWORD:-change-me}@db:5432/${POSTGRES_DB:-litellm}
    command:
      - --port
      - "4000"
    depends_on:
      - db
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-litellm}
      POSTGRES_USER: ${POSTGRES_USER:-litellm}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - litellm-db:/var/lib/postgresql/data
volumes:
  litellm-db:
`,
			EnvContent: "LITELLM_PORT=4000\nLITELLM_MASTER_KEY=change-me\nPOSTGRES_DB=litellm\nPOSTGRES_USER=litellm\nPOSTGRES_PASSWORD=change-me\n",
			Notes:      "Add provider API keys or a LiteLLM config file before routing production traffic through this proxy.",
		},
		{
			ID:          "flowise",
			Name:        "Flowise",
			Description: "Visual builder for AI chains, agents, and chatflows.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "flowiseai/flowise:latest",
			Tags:        []string{"ai", "agents", "workflow"},
			ComposeContent: `services:
  flowise:
    image: flowiseai/flowise:latest
    restart: unless-stopped
    ports:
      - "${FLOWISE_PORT:-3000}:3000"
    environment:
      PORT: 3000
      FLOWISE_USERNAME: ${FLOWISE_USERNAME:-admin}
      FLOWISE_PASSWORD: ${FLOWISE_PASSWORD:-change-me}
    volumes:
      - flowise-data:/root/.flowise
volumes:
  flowise-data:
`,
			EnvContent: "FLOWISE_PORT=3000\nFLOWISE_USERNAME=admin\nFLOWISE_PASSWORD=change-me\n",
			Notes:      "Change FLOWISE_PASSWORD before exposing the app.",
		},
		{
			ID:          "langflow",
			Name:        "Langflow",
			Description: "Visual LangChain-style AI app builder.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "langflowai/langflow:latest",
			Tags:        []string{"ai", "langchain", "workflow"},
			ComposeContent: `services:
  langflow:
    image: langflowai/langflow:latest
    restart: unless-stopped
    ports:
      - "${LANGFLOW_PORT:-7860}:7860"
    environment:
      LANGFLOW_DATABASE_URL: sqlite:////app/langflow/langflow.db
    volumes:
      - langflow-data:/app/langflow
volumes:
  langflow-data:
`,
			EnvContent: "LANGFLOW_PORT=7860\n",
		},
		{
			ID:          "localai",
			Name:        "LocalAI",
			Description: "OpenAI-compatible local model API.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "localai/localai:latest-aio-cpu",
			Tags:        []string{"ai", "openai-compatible", "models"},
			ComposeContent: `services:
  localai:
    image: localai/localai:latest-aio-cpu
    restart: unless-stopped
    ports:
      - "${LOCALAI_PORT:-8080}:8080"
    environment:
      DEBUG: "${LOCALAI_DEBUG:-false}"
      MODELS_PATH: /models
    volumes:
      - localai-models:/models
volumes:
  localai-models:
`,
			EnvContent: "LOCALAI_PORT=8080\nLOCALAI_DEBUG=false\n",
			Notes:      "CPU image is portable but slower. Swap the image/tag for a GPU build when the host supports it.",
		},
		{
			ID:          "open-webui",
			Name:        "Open WebUI",
			Description: "Web chat interface for Ollama and OpenAI-compatible APIs.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "ghcr.io/open-webui/open-webui:main",
			Tags:        []string{"ai", "chat", "ollama"},
			ComposeContent: `services:
  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    restart: unless-stopped
    ports:
      - "${OPEN_WEBUI_PORT:-3000}:8080"
    environment:
      OLLAMA_BASE_URL: ${OLLAMA_BASE_URL:-http://host.docker.internal:11434}
    extra_hosts:
      - host.docker.internal:host-gateway
    volumes:
      - open-webui-data:/app/backend/data
volumes:
  open-webui-data:
`,
			EnvContent: "OPEN_WEBUI_PORT=3000\nOLLAMA_BASE_URL=http://host.docker.internal:11434\n",
			Notes:      "Use this when Ollama already runs on the host or in another project.",
		},
		{
			ID:          "qdrant",
			Name:        "Qdrant",
			Description: "Vector database for semantic search and RAG.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "qdrant/qdrant:latest",
			Tags:        []string{"ai", "vector-db", "rag"},
			ComposeContent: `services:
  qdrant:
    image: qdrant/qdrant:latest
    restart: unless-stopped
    ports:
      - "${QDRANT_HTTP_PORT:-6333}:6333"
      - "${QDRANT_GRPC_PORT:-6334}:6334"
    volumes:
      - qdrant-storage:/qdrant/storage
volumes:
  qdrant-storage:
`,
			EnvContent: "QDRANT_HTTP_PORT=6333\nQDRANT_GRPC_PORT=6334\n",
		},
		{
			ID:          "weaviate",
			Name:        "Weaviate",
			Description: "Vector database with optional module support.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "semitechnologies/weaviate:latest",
			Tags:        []string{"ai", "vector-db", "rag", "search"},
			ComposeContent: `services:
  weaviate:
    image: semitechnologies/weaviate:latest
    restart: unless-stopped
    ports:
      - "${WEAVIATE_PORT:-8080}:8080"
      - "${WEAVIATE_GRPC_PORT:-50051}:50051"
    environment:
      QUERY_DEFAULTS_LIMIT: 25
      AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED: "${WEAVIATE_ANONYMOUS_ACCESS:-true}"
      PERSISTENCE_DATA_PATH: /var/lib/weaviate
      DEFAULT_VECTORIZER_MODULE: none
      CLUSTER_HOSTNAME: node1
    volumes:
      - weaviate-data:/var/lib/weaviate
volumes:
  weaviate-data:
`,
			EnvContent: "WEAVIATE_PORT=8080\nWEAVIATE_GRPC_PORT=50051\nWEAVIATE_ANONYMOUS_ACCESS=true\n",
			Notes:      "Turn off anonymous access and configure authentication before exposing Weaviate outside a trusted network.",
		},
		{
			ID:          "chroma",
			Name:        "Chroma",
			Description: "Embeddings database for local AI apps.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "chromadb/chroma:latest",
			Tags:        []string{"ai", "vector-db", "embeddings"},
			ComposeContent: `services:
  chroma:
    image: chromadb/chroma:latest
    restart: unless-stopped
    ports:
      - "${CHROMA_PORT:-8000}:8000"
    volumes:
      - chroma-data:/chroma/chroma
volumes:
  chroma-data:
`,
			EnvContent: "CHROMA_PORT=8000\n",
		},
		{
			ID:          "lobe-chat",
			Name:        "LobeChat",
			Description: "Polished multi-provider AI chat interface.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "lobehub/lobe-chat:latest",
			Tags:        []string{"ai", "chat", "frontend"},
			ComposeContent: `services:
  lobe-chat:
    image: lobehub/lobe-chat:latest
    restart: unless-stopped
    ports:
      - "${LOBE_CHAT_PORT:-3210}:3210"
    environment:
      ACCESS_CODE: ${LOBE_ACCESS_CODE:-change-me}
volumes: {}
`,
			EnvContent: "LOBE_CHAT_PORT=3210\nLOBE_ACCESS_CODE=change-me\n",
			Notes:      "Set provider API keys in the app or add environment variables for your chosen provider.",
		},
		{
			ID:          "whisper-asr",
			Name:        "Whisper ASR",
			Description: "Speech-to-text API using Whisper models.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "onerahmet/openai-whisper-asr-webservice:latest",
			Tags:        []string{"ai", "transcription", "audio"},
			ComposeContent: `services:
  whisper-asr:
    image: onerahmet/openai-whisper-asr-webservice:latest
    restart: unless-stopped
    ports:
      - "${WHISPER_ASR_PORT:-9000}:9000"
    environment:
      ASR_MODEL: ${ASR_MODEL:-base}
      ASR_ENGINE: ${ASR_ENGINE:-openai_whisper}
    volumes:
      - whisper-cache:/root/.cache/whisper
volumes:
  whisper-cache:
`,
			EnvContent: "WHISPER_ASR_PORT=9000\nASR_MODEL=base\nASR_ENGINE=openai_whisper\n",
			Notes:      "Larger models improve quality but need more CPU/RAM and take longer to download.",
		},
		{
			ID:          "comfyui",
			Name:        "ComfyUI",
			Description: "Node-based Stable Diffusion image generation UI.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "yanwk/comfyui-boot:latest",
			Tags:        []string{"ai", "images", "stable-diffusion"},
			ComposeContent: `services:
  comfyui:
    image: yanwk/comfyui-boot:latest
    restart: unless-stopped
    ports:
      - "${COMFYUI_PORT:-8188}:8188"
    volumes:
      - comfyui-storage:/root
volumes:
  comfyui-storage:
`,
			EnvContent: "COMFYUI_PORT=8188\n",
			Notes:      "Image generation is much better with GPU support. Add device/runtime settings manually for your host.",
		},
		{
			ID:          "jupyter-pytorch",
			Name:        "JupyterLab PyTorch",
			Description: "Notebook workspace with PyTorch for AI experiments.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "quay.io/jupyter/pytorch-notebook:latest",
			Tags:        []string{"ai", "notebooks", "pytorch", "devtools"},
			ComposeContent: `services:
  jupyter:
    image: quay.io/jupyter/pytorch-notebook:latest
    restart: unless-stopped
    ports:
      - "${JUPYTER_PORT:-8888}:8888"
    environment:
      JUPYTER_TOKEN: ${JUPYTER_TOKEN:-change-me}
    volumes:
      - jupyter-work:/home/jovyan/work
volumes:
  jupyter-work:
`,
			EnvContent: "JUPYTER_PORT=8888\nJUPYTER_TOKEN=change-me\n",
			Notes:      "Use a strong JUPYTER_TOKEN and add GPU runtime settings manually if the host supports CUDA.",
		},
		{
			ID:          "automatic1111",
			Name:        "AUTOMATIC1111",
			Description: "Stable Diffusion web UI starter.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "ghcr.io/ai-dock/stable-diffusion-webui:latest-cpu",
			Tags:        []string{"ai", "images", "stable-diffusion"},
			ComposeContent: `services:
  stable-diffusion-webui:
    image: ghcr.io/ai-dock/stable-diffusion-webui:latest-cpu
    restart: unless-stopped
    ports:
      - "${A1111_PORT:-7860}:7860"
    volumes:
      - stable-diffusion-data:/workspace
volumes:
  stable-diffusion-data:
`,
			EnvContent: "A1111_PORT=7860\n",
			Notes:      "This CPU starter is portable but slow. Replace the image/tag and add GPU runtime options for production image generation.",
		},
		{
			ID:          "open-webui-pipelines",
			Name:        "Open WebUI Pipelines",
			Description: "Plugin and function server for Open WebUI.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "ghcr.io/open-webui/pipelines:main",
			Tags:        []string{"ai", "plugins", "open-webui"},
			ComposeContent: `services:
  pipelines:
    image: ghcr.io/open-webui/pipelines:main
    restart: unless-stopped
    ports:
      - "${PIPELINES_PORT:-9099}:9099"
    volumes:
      - pipelines-data:/app/pipelines
volumes:
  pipelines-data:
`,
			EnvContent: "PIPELINES_PORT=9099\n",
			Notes:      "Connect this endpoint from Open WebUI after startup.",
		},
		{
			ID:          "searxng",
			Name:        "SearXNG",
			Description: "Private metasearch engine useful for AI research workflows.",
			Category:    "ai",
			Source:      "portainer-style",
			Image:       "searxng/searxng:latest",
			Tags:        []string{"ai", "search", "privacy"},
			ComposeContent: `services:
  searxng:
    image: searxng/searxng:latest
    restart: unless-stopped
    ports:
      - "${SEARXNG_PORT:-8080}:8080"
    environment:
      BASE_URL: ${SEARXNG_BASE_URL:-http://localhost:8080/}
    volumes:
      - searxng-data:/etc/searxng
volumes:
  searxng-data:
`,
			EnvContent: "SEARXNG_PORT=8080\nSEARXNG_BASE_URL=http://localhost:8080/\n",
			Notes:      "Useful as a private search backend for research and agent workflows.",
		},
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	return templates
}

func GetBuiltinStackTemplate(id string) (StackTemplate, bool) {
	id = strings.TrimSpace(id)
	for _, template := range BuiltinStackTemplates() {
		if template.ID == id {
			return template, true
		}
	}
	return StackTemplate{}, false
}

func RenderStackTemplate(id string) (CreateProjectRequest, error) {
	template, ok := GetBuiltinStackTemplate(id)
	if !ok {
		return CreateProjectRequest{}, fmt.Errorf("template not found: %s", id)
	}
	return CreateProjectRequest{
		Name:           template.ID,
		ComposeContent: template.ComposeContent,
		EnvContent:     template.EnvContent,
	}, nil
}
