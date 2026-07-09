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
	Subcategory    string   `json:"subcategory,omitempty"`
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
      - web-html:/usr/share/nginx/html
volumes:
  web-html:
`,
			EnvContent: "WEB_PORT=8080\n",
			Notes: "Drop files into the web-html volume or replace it with a `./html:/usr/share/nginx/html:ro` bind mount for host-side editing.",
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
    command: ["caddy", "file-server", "--listen", ":80"]
    ports:
      - "${HTTP_PORT:-80}:80"
      - "${HTTPS_PORT:-443}:443"
    volumes:
      - caddy-data:/data
      - caddy-config:/config
volumes:
  caddy-data:
  caddy-config:
`,
			EnvContent: "HTTP_PORT=80\nHTTPS_PORT=443\n",
			Notes: "Starts with a built-in file server. For reverse proxy or custom config, create a Caddyfile, add a `./Caddyfile:/etc/caddy/Caddyfile:ro` volume mount, and remove the command line.",
		},
		{
			ID:          "nginx-proxy-manager",
			Name:        "Nginx Proxy Manager",
			Description: "GUI reverse proxy with free SSL via Let's Encrypt. Manage proxy hosts, redirections, streams, and certificates from a browser.",
			Category:    "proxy",
			Source:      "community",
			Image:       "jc21/nginx-proxy-manager:latest",
			Tags:        []string{"proxy", "ssl", "letsencrypt", "reverse-proxy", "gui"},
			ComposeContent: `services:
  npm:
    image: jc21/nginx-proxy-manager:latest
    restart: unless-stopped
    ports:
      - "${NPM_HTTP_PORT:-80}:80"
      - "${NPM_HTTPS_PORT:-443}:443"
      - "${NPM_ADMIN_PORT:-81}:81"
    volumes:
      - npm-data:/data
      - npm-letsencrypt:/etc/letsencrypt
volumes:
  npm-data:
  npm-letsencrypt:
`,
			EnvContent: `NPM_HTTP_PORT=80
NPM_HTTPS_PORT=443
NPM_ADMIN_PORT=81
`,
			Notes: "Default admin login: admin@example.com / changeme. Change it immediately after first login. If Stack Manager's web container also binds port 80, change WEB_HTTP_PORT in Stack Manager's .env to avoid a conflict, or set it to 0 to disable.",
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
			Subcategory: "llm-inference",
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
			Subcategory: "llm-inference",
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
			Subcategory: "llm-inference",
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
			Subcategory: "llm-inference",
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
			Subcategory: "llm-inference",
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
			Subcategory: "llm-inference",
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
			Subcategory: "workflow-rag",
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
			Subcategory: "workflow-rag",
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
			Subcategory: "llm-inference",
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
			Subcategory: "llm-inference",
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
			Subcategory: "vector-db",
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
			Subcategory: "vector-db",
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
			Subcategory: "vector-db",
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
			Subcategory: "llm-inference",
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
			Subcategory: "voice-speech",
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
			Subcategory: "image-generation",
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
			Subcategory: "workflow-rag",
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
			Subcategory: "image-generation",
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
			Subcategory: "workflow-rag",
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
			Subcategory: "search",
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
	templates = append(templates, categoryExpansionStackTemplates()...)
	templates = append(templates, catalogFillTemplates()...)
	templates = append(templates, vettedAIStackTemplates()...)
	templates = append(templates, personalAgentStackTemplates()...)
	templates = append(templates, linuxserverMediaTemplates()...)
	templates = append(templates, gamingTemplates()...)
	templates = append(templates, remoteDesktopTemplates()...)
	// Deduplicate by ID — first occurrence wins so the primary
	// templates.go definition takes precedence over fill templates.
	seen := map[string]bool{}
	deduped := make([]StackTemplate, 0, len(templates))
	for _, t := range templates {
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		deduped = append(deduped, t)
	}
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].Name < deduped[j].Name
	})
	return deduped
}

func categoryExpansionStackTemplates() []StackTemplate {
	return []StackTemplate{
		{
			ID:          "apache-httpd",
			Name:        "Apache HTTP Server",
			Description: "Apache static web server with a bind-mounted public directory.",
			Category:    "web",
			Source:      "docker-hub-official",
			Image:       "httpd:2.4-alpine",
			Tags:        []string{"web", "static", "apache"},
			ComposeContent: `services:
  apache:
    image: httpd:2.4-alpine
    restart: unless-stopped
    ports:
      - "${APACHE_PORT:-8080}:80"
    volumes:
      - ./public:/usr/local/apache2/htdocs:ro
`,
			EnvContent: "APACHE_PORT=8080\n",
			Notes:      "Create a public directory beside compose.yml before starting, or edit the bind mount.",
		},
		{
			ID:          "authelia",
			Name:        "Authelia",
			Description: "Single sign-on and two-factor authentication portal.",
			Category:    "security",
			Source:      "official-docs",
			Image:       "authelia/authelia:latest",
			Tags:        []string{"security", "sso", "2fa"},
			ComposeContent: `services:
  authelia:
    image: authelia/authelia:latest
    restart: unless-stopped
    ports:
      - "${AUTHELIA_PORT:-9091}:9091"
    volumes:
      - ./config:/config
    environment:
      TZ: ${TZ:-UTC}
`,
			EnvContent: "AUTHELIA_PORT=9091\nTZ=UTC\n",
			Notes:      "Create config/configuration.yml before starting. Authelia will not boot without a valid configuration.",
		},
		{
			ID:          "audiobookshelf",
			Name:        "Audiobookshelf",
			Description: "Self-hosted audiobook and podcast server.",
			Category:    "media",
			Source:      "official-docs",
			Image:       "ghcr.io/advplyr/audiobookshelf:latest",
			Tags:        []string{"media", "audiobooks", "podcasts"},
			ComposeContent: `services:
  audiobookshelf:
    image: ghcr.io/advplyr/audiobookshelf:latest
    restart: unless-stopped
    ports:
      - "${AUDIOBOOKSHELF_PORT:-13378}:80"
    volumes:
      - audiobookshelf-config:/config
      - audiobookshelf-metadata:/metadata
      - ./audiobooks:/audiobooks
      - ./podcasts:/podcasts
volumes:
  audiobookshelf-config:
  audiobookshelf-metadata:
`,
			EnvContent: "AUDIOBOOKSHELF_PORT=13378\n",
			Notes:      "Create audiobooks and podcasts directories beside compose.yml or edit the bind mounts.",
		},
		{
			ID:          "beanstalkd",
			Name:        "Beanstalkd",
			Description: "Simple fast work queue service.",
			Category:    "queue",
			Source:      "docker-hub",
			Image:       "schickling/beanstalkd:latest",
			Tags:        []string{"queue", "jobs", "work-queue"},
			ComposeContent: `services:
  beanstalkd:
    image: schickling/beanstalkd:latest
    restart: unless-stopped
    ports:
      - "${BEANSTALKD_PORT:-11300}:11300"
`,
			EnvContent: "BEANSTALKD_PORT=11300\n",
		},
		{
			ID:          "beszel",
			Name:        "Beszel",
			Description: "Lightweight server monitoring hub with a local agent.",
			Category:    "monitoring",
			Source:      "official-docs",
			Image:       "henrygd/beszel:latest",
			Tags:        []string{"monitoring", "metrics", "servers"},
			ComposeContent: `services:
  beszel:
    image: henrygd/beszel:latest
    restart: unless-stopped
    ports:
      - "${BESZEL_PORT:-8090}:8090"
    environment:
      APP_URL: ${BESZEL_APP_URL:-http://localhost:8090}
    volumes:
      - beszel-data:/beszel_data
      - beszel-socket:/beszel_socket
  beszel-agent:
    image: henrygd/beszel-agent:latest
    restart: unless-stopped
    network_mode: host
    environment:
      LISTEN: /beszel_socket/beszel.sock
      HUB_URL: ${BESZEL_APP_URL:-http://localhost:8090}
      TOKEN: ${BESZEL_AGENT_TOKEN:-}
      KEY: ${BESZEL_AGENT_KEY:-}
    volumes:
      - beszel-agent-data:/var/lib/beszel-agent
      - beszel-socket:/beszel_socket
      - /var/run/docker.sock:/var/run/docker.sock:ro
volumes:
  beszel-data:
  beszel-agent-data:
  beszel-socket:
`,
			EnvContent: "BESZEL_PORT=8090\nBESZEL_APP_URL=http://localhost:8090\nBESZEL_AGENT_TOKEN=\nBESZEL_AGENT_KEY=\n",
			Notes:      "Create the admin user first, then fill BESZEL_AGENT_TOKEN and BESZEL_AGENT_KEY from the Add System flow.",
		},
		{
			ID:          "calibre-web",
			Name:        "Calibre-Web",
			Description: "Web library for ebooks backed by a Calibre database.",
			Category:    "media",
			Source:      "linuxserver-docs",
			Image:       "lscr.io/linuxserver/calibre-web:latest",
			Tags:        []string{"media", "ebooks", "library"},
			ComposeContent: `services:
  calibre-web:
    image: lscr.io/linuxserver/calibre-web:latest
    restart: unless-stopped
    ports:
      - "${CALIBRE_WEB_PORT:-8083}:8083"
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
    volumes:
      - calibre-web-config:/config
      - ./books:/books
volumes:
  calibre-web-config:
`,
			EnvContent: "CALIBRE_WEB_PORT=8083\nPUID=1000\nPGID=1000\nTZ=UTC\n",
			Notes:      "Create a books directory with a Calibre library database or edit the bind mount.",
		},
		{
			ID:          "caddy-static",
			Name:        "Caddy Static Site",
			Description: "Caddy web server for static files.",
			Category:    "web",
			Source:      "docker-hub-official",
			Image:       "caddy:2-alpine",
			Tags:        []string{"web", "static", "caddy"},
			ComposeContent: `services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    command: ["caddy", "file-server", "--listen", ":80", "--root", "/usr/share/caddy"]
    ports:
      - "${CADDY_STATIC_PORT:-8080}:80"
    volumes:
      - caddy-site:/usr/share/caddy
      - caddy-data:/data
volumes:
  caddy-site:
  caddy-data:
`,
			EnvContent: "CADDY_STATIC_PORT=8080\n",
			Notes: "Drop files into the caddy-site volume or replace it with a `./site:/usr/share/caddy:ro` bind mount.",
		},
		{
			ID:          "crowdsec",
			Name:        "CrowdSec",
			Description: "Collaborative intrusion detection and remediation engine.",
			Category:    "security",
			Source:      "official-docs",
			Image:       "crowdsecurity/crowdsec:latest",
			Tags:        []string{"security", "ids", "firewall"},
			ComposeContent: `services:
  crowdsec:
    image: crowdsecurity/crowdsec:latest
    restart: unless-stopped
    environment:
      COLLECTIONS: ${CROWDSEC_COLLECTIONS:-crowdsecurity/linux}
      GID: ${DOCKER_GID:-998}
    volumes:
      - crowdsec-config:/etc/crowdsec
      - crowdsec-data:/var/lib/crowdsec/data
      - /var/log:/var/log:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
volumes:
  crowdsec-config:
  crowdsec-data:
`,
			EnvContent: "CROWDSEC_COLLECTIONS=crowdsecurity/linux\nDOCKER_GID=998\n",
			Notes:      "Adjust DOCKER_GID to the host Docker socket group if CrowdSec needs container log access.",
		},
		{
			ID:          "dashy",
			Name:        "Dashy",
			Description: "Self-hosted start page and service dashboard.",
			Category:    "management",
			Source:      "docker-hub",
			Image:       "lissy93/dashy:latest",
			Tags:        []string{"dashboard", "management", "homepage"},
			ComposeContent: `services:
  dashy:
    image: lissy93/dashy:latest
    restart: unless-stopped
    ports:
      - "${DASHY_PORT:-8080}:8080"
    volumes:
      - ./conf.yml:/app/user-data/conf.yml
`,
			EnvContent: "DASHY_PORT=8080\n",
			Notes:      "Create conf.yml beside compose.yml before starting, or remove the bind mount for the image defaults.",
		},
		{
			ID:          "diun",
			Name:        "Diun",
			Description: "Docker image update notification service.",
			Category:    "automation",
			Source:      "official-docs",
			Image:       "crazymax/diun:latest",
			Tags:        []string{"automation", "updates", "notifications"},
			ComposeContent: `services:
  diun:
    image: crazymax/diun:latest
    restart: unless-stopped
    command: serve
    environment:
      TZ: ${TZ:-UTC}
      LOG_LEVEL: ${LOG_LEVEL:-info}
      DIUN_PROVIDERS_DOCKER: "true"
      DIUN_PROVIDERS_DOCKER_WATCHBYDEFAULT: "true"
    volumes:
      - diun-data:/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
volumes:
  diun-data:
`,
			EnvContent: "TZ=UTC\nLOG_LEVEL=info\n",
			Notes:      "Configure notification targets in environment variables before relying on alerts.",
		},
		{
			ID:          "directus",
			Name:        "Directus",
			Description: "Headless CMS and API layer backed by PostgreSQL.",
			Category:    "cms",
			Source:      "official-docs",
			Image:       "directus/directus:latest",
			Tags:        []string{"cms", "headless", "api"},
			ComposeContent: `services:
  directus:
    image: directus/directus:latest
    restart: unless-stopped
    ports:
      - "${DIRECTUS_PORT:-8055}:8055"
    environment:
      KEY: ${DIRECTUS_KEY:?set DIRECTUS_KEY}
      SECRET: ${DIRECTUS_SECRET:?set DIRECTUS_SECRET}
      DB_CLIENT: pg
      DB_HOST: db
      DB_PORT: 5432
      DB_DATABASE: ${POSTGRES_DB:-directus}
      DB_USER: ${POSTGRES_USER:-directus}
      DB_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      ADMIN_EMAIL: ${DIRECTUS_ADMIN_EMAIL:?set DIRECTUS_ADMIN_EMAIL}
      ADMIN_PASSWORD: ${DIRECTUS_ADMIN_PASSWORD:?set DIRECTUS_ADMIN_PASSWORD}
    depends_on:
      - db
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-directus}
      POSTGRES_USER: ${POSTGRES_USER:-directus}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - directus-db:/var/lib/postgresql/data
volumes:
  directus-db:
`,
			EnvContent: "DIRECTUS_PORT=8055\nDIRECTUS_KEY=\nDIRECTUS_SECRET=\nDIRECTUS_ADMIN_EMAIL=\nDIRECTUS_ADMIN_PASSWORD=\nPOSTGRES_DB=directus\nPOSTGRES_USER=directus\nPOSTGRES_PASSWORD=\n",
			Notes:      "Generate DIRECTUS_KEY, DIRECTUS_SECRET, DIRECTUS_ADMIN_PASSWORD, and POSTGRES_PASSWORD before starting.",
		},
		{
			ID:          "dockge",
			Name:        "Dockge",
			Description: "Docker Compose stack manager.",
			Category:    "management",
			Source:      "upstream-github",
			Image:       "louislam/dockge:1",
			Tags:        []string{"management", "docker", "compose"},
			ComposeContent: `services:
  dockge:
    image: louislam/dockge:1
    restart: unless-stopped
    ports:
      - "${DOCKGE_PORT:-5001}:5001"
    environment:
      DOCKGE_STACKS_DIR: ${DOCKGE_STACKS_DIR:-/opt/stacks}
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - dockge-data:/app/data
      - ${DOCKGE_STACKS_DIR:-/opt/stacks}:${DOCKGE_STACKS_DIR:-/opt/stacks}
volumes:
  dockge-data:
`,
			EnvContent: "DOCKGE_PORT=5001\nDOCKGE_STACKS_DIR=/opt/stacks\nPUID=1000\nPGID=1000\n",
			Notes:      "Set DOCKGE_STACKS_DIR to the host directory containing stacks before starting.",
		},
		{
			ID:          "dokuwiki",
			Name:        "DokuWiki",
			Description: "File-backed wiki with no external database.",
			Category:    "docs",
			Source:      "linuxserver-docs",
			Image:       "lscr.io/linuxserver/dokuwiki:latest",
			Tags:        []string{"docs", "wiki", "knowledge-base"},
			ComposeContent: `services:
  dokuwiki:
    image: lscr.io/linuxserver/dokuwiki:latest
    restart: unless-stopped
    ports:
      - "${DOKUWIKI_PORT:-8080}:80"
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
    volumes:
      - dokuwiki-config:/config
volumes:
  dokuwiki-config:
`,
			EnvContent: "DOKUWIKI_PORT=8080\nPUID=1000\nPGID=1000\nTZ=UTC\n",
		},
		{
			ID:          "drupal-postgres",
			Name:        "Drupal + PostgreSQL",
			Description: "Drupal CMS with PostgreSQL persistence.",
			Category:    "cms",
			Source:      "docker-hub-official",
			Image:       "drupal:10-apache",
			Tags:        []string{"cms", "drupal", "postgres"},
			ComposeContent: `services:
  drupal:
    image: drupal:10-apache
    restart: unless-stopped
    ports:
      - "${DRUPAL_PORT:-8080}:80"
    volumes:
      - drupal-modules:/var/www/html/modules
      - drupal-profiles:/var/www/html/profiles
      - drupal-sites:/var/www/html/sites
      - drupal-themes:/var/www/html/themes
    depends_on:
      - db
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-drupal}
      POSTGRES_USER: ${POSTGRES_USER:-drupal}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - drupal-db:/var/lib/postgresql/data
volumes:
  drupal-modules:
  drupal-profiles:
  drupal-sites:
  drupal-themes:
  drupal-db:
`,
			EnvContent: "DRUPAL_PORT=8080\nPOSTGRES_DB=drupal\nPOSTGRES_USER=drupal\nPOSTGRES_PASSWORD=\n",
			Notes:      "Set POSTGRES_PASSWORD before starting and enter the same database values in the Drupal installer.",
		},
		{
			ID:          "duplicati",
			Name:        "Duplicati",
			Description: "Web-managed encrypted backup tool for files and folders.",
			Category:    "files",
			Source:      "linuxserver-docs",
			Image:       "lscr.io/linuxserver/duplicati:latest",
			Tags:        []string{"files", "backup", "sync"},
			ComposeContent: `services:
  duplicati:
    image: lscr.io/linuxserver/duplicati:latest
    restart: unless-stopped
    ports:
      - "${DUPLICATI_PORT:-8200}:8200"
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
    volumes:
      - duplicati-config:/config
      - ./backups:/backups
      - ./source:/source:ro
volumes:
  duplicati-config:
`,
			EnvContent: "DUPLICATI_PORT=8200\nPUID=1000\nPGID=1000\nTZ=UTC\n",
			Notes:      "Create backups and source directories or edit the bind mounts before starting.",
		},
		{
			ID:          "emby",
			Name:        "Emby",
			Description: "Personal media server for video, music, and photos.",
			Category:    "media",
			Source:      "linuxserver-docs",
			Image:       "lscr.io/linuxserver/emby:latest",
			Tags:        []string{"media", "video", "music"},
			ComposeContent: `services:
  emby:
    image: lscr.io/linuxserver/emby:latest
    restart: unless-stopped
    ports:
      - "${EMBY_HTTP_PORT:-8096}:8096"
      - "${EMBY_HTTPS_PORT:-8920}:8920"
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
    volumes:
      - emby-config:/config
      - ./media:/data/media:ro
volumes:
  emby-config:
`,
			EnvContent: "EMBY_HTTP_PORT=8096\nEMBY_HTTPS_PORT=8920\nPUID=1000\nPGID=1000\nTZ=UTC\n",
			Notes:      "Create a media directory or edit the bind mount before starting.",
		},
		{
			ID:          "filebrowser",
			Name:        "File Browser",
			Description: "Web file manager for a mounted directory.",
			Category:    "files",
			Source:      "official-docs",
			Image:       "filebrowser/filebrowser:latest",
			Tags:        []string{"files", "browser", "manager"},
			ComposeContent: `services:
  filebrowser:
    image: filebrowser/filebrowser:latest
    restart: unless-stopped
    ports:
      - "${FILEBROWSER_PORT:-8080}:80"
    volumes:
      - ./files:/srv
      - filebrowser-db:/database
      - filebrowser-config:/config
volumes:
  filebrowser-db:
  filebrowser-config:
`,
			EnvContent: "FILEBROWSER_PORT=8080\n",
			Notes:      "Create a files directory before starting. The initial admin password is generated and printed in container logs.",
		},
		{
			ID:          "forgejo",
			Name:        "Forgejo",
			Description: "Self-hosted Git forge with SSH and web access.",
			Category:    "devtools",
			Source:      "official-docs",
			Image:       "codeberg.org/forgejo/forgejo:9",
			Tags:        []string{"git", "devtools", "forge"},
			ComposeContent: `services:
  forgejo:
    image: codeberg.org/forgejo/forgejo:9
    restart: unless-stopped
    environment:
      USER_UID: ${USER_UID:-1000}
      USER_GID: ${USER_GID:-1000}
    ports:
      - "${FORGEJO_WEB_PORT:-3000}:3000"
      - "${FORGEJO_SSH_PORT:-2222}:22"
    volumes:
      - forgejo-data:/data
volumes:
  forgejo-data:
`,
			EnvContent: "USER_UID=1000\nUSER_GID=1000\nFORGEJO_WEB_PORT=3000\nFORGEJO_SSH_PORT=2222\n",
		},
		{
			ID:          "gatus",
			Name:        "Gatus",
			Description: "Developer-friendly status page and health monitor.",
			Category:    "monitoring",
			Source:      "official-docs",
			Image:       "twinproduction/gatus:latest",
			Tags:        []string{"monitoring", "status", "healthchecks"},
			ComposeContent: `services:
  gatus:
    image: twinproduction/gatus:latest
    restart: unless-stopped
    ports:
      - "${GATUS_PORT:-8080}:8080"
    volumes:
      - ./config:/config
`,
			EnvContent: "GATUS_PORT=8080\n",
			Notes:      "Create config/config.yaml before starting, or edit the bind mount.",
		},
		{
			ID:          "ghost-mysql",
			Name:        "Ghost + MySQL",
			Description: "Ghost publishing CMS with MySQL persistence.",
			Category:    "cms",
			Source:      "docker-hub-official",
			Image:       "ghost:5-alpine",
			Tags:        []string{"cms", "publishing", "mysql"},
			ComposeContent: `services:
  ghost:
    image: ghost:5-alpine
    restart: unless-stopped
    ports:
      - "${GHOST_PORT:-2368}:2368"
    environment:
      url: ${GHOST_URL:-http://localhost:2368}
      database__client: mysql
      database__connection__host: db
      database__connection__user: ${MYSQL_USER:-ghost}
      database__connection__password: ${MYSQL_PASSWORD:?set MYSQL_PASSWORD}
      database__connection__database: ${MYSQL_DATABASE:-ghost}
    volumes:
      - ghost-content:/var/lib/ghost/content
    depends_on:
      - db
  db:
    image: mysql:8.4
    restart: unless-stopped
    environment:
      MYSQL_DATABASE: ${MYSQL_DATABASE:-ghost}
      MYSQL_USER: ${MYSQL_USER:-ghost}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:?set MYSQL_PASSWORD}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:?set MYSQL_ROOT_PASSWORD}
    volumes:
      - ghost-db:/var/lib/mysql
volumes:
  ghost-content:
  ghost-db:
`,
			EnvContent: "GHOST_PORT=2368\nGHOST_URL=http://localhost:2368\nMYSQL_DATABASE=ghost\nMYSQL_USER=ghost\nMYSQL_PASSWORD=\nMYSQL_ROOT_PASSWORD=\n",
			Notes:      "Set MYSQL_PASSWORD and MYSQL_ROOT_PASSWORD before starting.",
		},
		{
			ID:          "grav",
			Name:        "Grav",
			Description: "Flat-file CMS with no database dependency.",
			Category:    "cms",
			Source:      "linuxserver-docs",
			Image:       "lscr.io/linuxserver/grav:latest",
			Tags:        []string{"cms", "flat-file", "website"},
			ComposeContent: `services:
  grav:
    image: lscr.io/linuxserver/grav:latest
    restart: unless-stopped
    ports:
      - "${GRAV_PORT:-8080}:80"
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
    volumes:
      - grav-config:/config
volumes:
  grav-config:
`,
			EnvContent: "GRAV_PORT=8080\nPUID=1000\nPGID=1000\nTZ=UTC\n",
		},
		{
			ID:          "hedgedoc",
			Name:        "HedgeDoc",
			Description: "Collaborative markdown notes and documentation editor.",
			Category:    "docs",
			Source:      "official-docs",
			Image:       "quay.io/hedgedoc/hedgedoc:latest",
			Tags:        []string{"docs", "markdown", "collaboration"},
			ComposeContent: `services:
  hedgedoc:
    image: quay.io/hedgedoc/hedgedoc:latest
    restart: unless-stopped
    ports:
      - "${HEDGEDOC_PORT:-3000}:3000"
    environment:
      CMD_DB_URL: postgres://${POSTGRES_USER:-hedgedoc}:${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB:-hedgedoc}
      CMD_DOMAIN: ${HEDGEDOC_DOMAIN:-localhost}
      CMD_PROTOCOL_USESSL: "${HEDGEDOC_USE_SSL:-false}"
      CMD_SESSION_SECRET: ${HEDGEDOC_SESSION_SECRET:?set HEDGEDOC_SESSION_SECRET}
    volumes:
      - hedgedoc-uploads:/hedgedoc/public/uploads
    depends_on:
      - db
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-hedgedoc}
      POSTGRES_USER: ${POSTGRES_USER:-hedgedoc}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - hedgedoc-db:/var/lib/postgresql/data
volumes:
  hedgedoc-uploads:
  hedgedoc-db:
`,
			EnvContent: "HEDGEDOC_PORT=3000\nHEDGEDOC_DOMAIN=localhost\nHEDGEDOC_USE_SSL=false\nHEDGEDOC_SESSION_SECRET=\nPOSTGRES_DB=hedgedoc\nPOSTGRES_USER=hedgedoc\nPOSTGRES_PASSWORD=\n",
			Notes:      "Generate HEDGEDOC_SESSION_SECRET and POSTGRES_PASSWORD before starting.",
		},
		{
			ID:          "jenkins",
			Name:        "Jenkins",
			Description: "Automation server for builds and delivery workflows.",
			Category:    "devtools",
			Source:      "official-docs",
			Image:       "jenkins/jenkins:lts-jdk21",
			Tags:        []string{"devtools", "ci", "automation"},
			ComposeContent: `services:
  jenkins:
    image: jenkins/jenkins:lts-jdk21
    restart: unless-stopped
    ports:
      - "${JENKINS_PORT:-8080}:8080"
      - "${JENKINS_AGENT_PORT:-50000}:50000"
    volumes:
      - jenkins-home:/var/jenkins_home
volumes:
  jenkins-home:
`,
			EnvContent: "JENKINS_PORT=8080\nJENKINS_AGENT_PORT=50000\n",
			Notes:      "Read the initial admin password from container logs or /var/jenkins_home/secrets/initialAdminPassword.",
		},
		{
			ID:          "joomla-mariadb",
			Name:        "Joomla + MariaDB",
			Description: "Joomla CMS with MariaDB persistence.",
			Category:    "cms",
			Source:      "docker-hub-official",
			Image:       "joomla:latest",
			Tags:        []string{"cms", "joomla", "mariadb"},
			ComposeContent: `services:
  joomla:
    image: joomla:latest
    restart: unless-stopped
    ports:
      - "${JOOMLA_PORT:-8080}:80"
    environment:
      JOOMLA_DB_HOST: db
      JOOMLA_DB_NAME: ${MYSQL_DATABASE:-joomla}
      JOOMLA_DB_USER: ${MYSQL_USER:-joomla}
      JOOMLA_DB_PASSWORD: ${MYSQL_PASSWORD:?set MYSQL_PASSWORD}
    volumes:
      - joomla-data:/var/www/html
    depends_on:
      - db
  db:
    image: mariadb:11.4
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: ${MYSQL_DATABASE:-joomla}
      MARIADB_USER: ${MYSQL_USER:-joomla}
      MARIADB_PASSWORD: ${MYSQL_PASSWORD:?set MYSQL_PASSWORD}
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:?set MARIADB_ROOT_PASSWORD}
    volumes:
      - joomla-db:/var/lib/mysql
volumes:
  joomla-data:
  joomla-db:
`,
			EnvContent: "JOOMLA_PORT=8080\nMYSQL_DATABASE=joomla\nMYSQL_USER=joomla\nMYSQL_PASSWORD=\nMARIADB_ROOT_PASSWORD=\n",
			Notes:      "Set MYSQL_PASSWORD and MARIADB_ROOT_PASSWORD before starting.",
		},
		{
			ID:          "kafka-kraft",
			Name:        "Apache Kafka",
			Description: "Single-node Kafka broker using KRaft mode.",
			Category:    "queue",
			Source:      "official-docs",
			Image:       "apache/kafka:latest",
			Tags:        []string{"queue", "streaming", "kafka"},
			ComposeContent: `services:
  kafka:
    image: apache/kafka:latest
    restart: unless-stopped
    ports:
      - "${KAFKA_PORT:-9092}:9092"
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://${KAFKA_ADVERTISED_HOST:-localhost}:${KAFKA_PORT:-9092}
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: 0
    volumes:
      - kafka-data:/var/lib/kafka/data
volumes:
  kafka-data:
`,
			EnvContent: "KAFKA_PORT=9092\nKAFKA_ADVERTISED_HOST=localhost\n",
		},
		{
			ID:          "keycloak-postgres",
			Name:        "Keycloak + PostgreSQL",
			Description: "Identity and access management server with PostgreSQL.",
			Category:    "security",
			Source:      "official-docs",
			Image:       "quay.io/keycloak/keycloak:latest",
			Tags:        []string{"security", "identity", "sso"},
			ComposeContent: `services:
  keycloak:
    image: quay.io/keycloak/keycloak:latest
    restart: unless-stopped
    command: ["start", "--http-enabled=true", "--hostname=${KEYCLOAK_HOSTNAME:-localhost}"]
    ports:
      - "${KEYCLOAK_PORT:-8080}:8080"
    environment:
      KC_DB: postgres
      KC_DB_URL: jdbc:postgresql://db:5432/${POSTGRES_DB:-keycloak}
      KC_DB_USERNAME: ${POSTGRES_USER:-keycloak}
      KC_DB_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      KC_BOOTSTRAP_ADMIN_USERNAME: ${KEYCLOAK_ADMIN_USER:-admin}
      KC_BOOTSTRAP_ADMIN_PASSWORD: ${KEYCLOAK_ADMIN_PASSWORD:?set KEYCLOAK_ADMIN_PASSWORD}
    depends_on:
      - db
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-keycloak}
      POSTGRES_USER: ${POSTGRES_USER:-keycloak}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - keycloak-db:/var/lib/postgresql/data
volumes:
  keycloak-db:
`,
			EnvContent: "KEYCLOAK_PORT=8080\nKEYCLOAK_HOSTNAME=localhost\nKEYCLOAK_ADMIN_USER=admin\nKEYCLOAK_ADMIN_PASSWORD=\nPOSTGRES_DB=keycloak\nPOSTGRES_USER=keycloak\nPOSTGRES_PASSWORD=\n",
			Notes:      "Set KEYCLOAK_ADMIN_PASSWORD and POSTGRES_PASSWORD before starting. Add TLS or a trusted reverse proxy before production use.",
		},
		{
			ID:          "mosquitto",
			Name:        "Eclipse Mosquitto",
			Description: "MQTT broker for IoT and event messaging.",
			Category:    "queue",
			Source:      "docker-hub-official",
			Image:       "eclipse-mosquitto:2",
			Tags:        []string{"queue", "mqtt", "iot"},
			ComposeContent: `services:
  mosquitto:
    image: eclipse-mosquitto:2
    restart: unless-stopped
    ports:
      - "${MOSQUITTO_PORT:-1883}:1883"
      - "${MOSQUITTO_WS_PORT:-9001}:9001"
    volumes:
      - ./config:/mosquitto/config
      - mosquitto-data:/mosquitto/data
      - mosquitto-log:/mosquitto/log
volumes:
  mosquitto-data:
  mosquitto-log:
`,
			EnvContent: "MOSQUITTO_PORT=1883\nMOSQUITTO_WS_PORT=9001\n",
			Notes:      "Create config/mosquitto.conf before starting. Avoid anonymous listeners on untrusted networks.",
		},
		{
			ID:          "nats",
			Name:        "NATS",
			Description: "Lightweight messaging system with JetStream persistence.",
			Category:    "queue",
			Source:      "official-docs",
			Image:       "nats:2-alpine",
			Tags:        []string{"queue", "messaging", "jetstream"},
			ComposeContent: `services:
  nats:
    image: nats:2-alpine
    restart: unless-stopped
    command: ["-js", "-sd", "/data", "-m", "8222"]
    ports:
      - "${NATS_CLIENT_PORT:-4222}:4222"
      - "${NATS_MONITOR_PORT:-8222}:8222"
    volumes:
      - nats-data:/data
volumes:
  nats-data:
`,
			EnvContent: "NATS_CLIENT_PORT=4222\nNATS_MONITOR_PORT=8222\n",
		},
		{
			ID:          "navidrome",
			Name:        "Navidrome",
			Description: "Self-hosted music streaming server.",
			Category:    "media",
			Source:      "official-docs",
			Image:       "deluan/navidrome:latest",
			Tags:        []string{"media", "music", "streaming"},
			ComposeContent: `services:
  navidrome:
    image: deluan/navidrome:latest
    restart: unless-stopped
    ports:
      - "${NAVIDROME_PORT:-4533}:4533"
    environment:
      ND_SCANSCHEDULE: ${NAVIDROME_SCAN_SCHEDULE:-1h}
      ND_LOGLEVEL: ${NAVIDROME_LOG_LEVEL:-info}
      ND_BASEURL: ${NAVIDROME_BASE_URL:-}
    volumes:
      - navidrome-data:/data
      - ./music:/music:ro
volumes:
  navidrome-data:
`,
			EnvContent: "NAVIDROME_PORT=4533\nNAVIDROME_SCAN_SCHEDULE=1h\nNAVIDROME_LOG_LEVEL=info\nNAVIDROME_BASE_URL=\n",
			Notes:      "Create a music directory or edit the bind mount before starting.",
		},
		{
			ID:          "netdata",
			Name:        "Netdata",
			Description: "Real-time host and container metrics dashboard.",
			Category:    "monitoring",
			Source:      "official-docs",
			Image:       "netdata/netdata:stable",
			Tags:        []string{"monitoring", "metrics", "containers"},
			ComposeContent: `services:
  netdata:
    image: netdata/netdata:stable
    restart: unless-stopped
    pid: host
    network_mode: host
    cap_add:
      - SYS_PTRACE
      - SYS_ADMIN
    security_opt:
      - apparmor:unconfined
    volumes:
      - netdata-config:/etc/netdata
      - netdata-lib:/var/lib/netdata
      - netdata-cache:/var/cache/netdata
      - /:/host/root:ro,rslave
      - /etc/passwd:/host/etc/passwd:ro
      - /etc/group:/host/etc/group:ro
      - /etc/localtime:/etc/localtime:ro
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /etc/os-release:/host/etc/os-release:ro
      - /var/log:/host/var/log:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
volumes:
  netdata-config:
  netdata-lib:
  netdata-cache:
`,
			Notes: "Netdata uses host networking and host mounts for full-node visibility. Review the mounts before deploying.",
		},
		{
			ID:          "nginx-unprivileged",
			Name:        "Nginx Unprivileged",
			Description: "Rootless-friendly Nginx static web server.",
			Category:    "web",
			Source:      "docker-hub",
			Image:       "nginxinc/nginx-unprivileged:stable-alpine",
			Tags:        []string{"web", "static", "nginx", "non-root"},
			ComposeContent: `services:
  nginx:
    image: nginxinc/nginx-unprivileged:stable-alpine
    restart: unless-stopped
    ports:
      - "${NGINX_UNPRIVILEGED_PORT:-8080}:8080"
    volumes:
      - ./html:/usr/share/nginx/html:ro
`,
			EnvContent: "NGINX_UNPRIVILEGED_PORT=8080\n",
			Notes:      "Create an html directory beside compose.yml before starting, or edit the bind mount.",
		},
		{
			ID:          "node-red",
			Name:        "Node-RED",
			Description: "Flow-based automation and integration builder.",
			Category:    "automation",
			Source:      "official-docs",
			Image:       "nodered/node-red:latest",
			Tags:        []string{"automation", "iot", "flows"},
			ComposeContent: `services:
  node-red:
    image: nodered/node-red:latest
    restart: unless-stopped
    ports:
      - "${NODE_RED_PORT:-1880}:1880"
    environment:
      TZ: ${TZ:-UTC}
    volumes:
      - node-red-data:/data
volumes:
  node-red-data:
`,
			EnvContent: "NODE_RED_PORT=1880\nTZ=UTC\n",
			Notes:      "Set a Node-RED credential secret in settings.js before storing sensitive flow credentials.",
		},
		{
			ID:          "php-apache",
			Name:        "PHP Apache",
			Description: "PHP-enabled Apache web app starter.",
			Category:    "web",
			Source:      "docker-hub-official",
			Image:       "php:8.3-apache",
			Tags:        []string{"web", "php", "apache"},
			ComposeContent: `services:
  php:
    image: php:8.3-apache
    restart: unless-stopped
    ports:
      - "${PHP_APACHE_PORT:-8080}:80"
    volumes:
      - ./app:/var/www/html
`,
			EnvContent: "PHP_APACHE_PORT=8080\n",
			Notes:      "Create an app directory with PHP files before starting, or edit the bind mount.",
		},
		{
			ID:          "pihole",
			Name:        "Pi-hole",
			Description: "DNS sinkhole and local network ad blocker.",
			Category:    "security",
			Source:      "upstream-github",
			Image:       "pihole/pihole:latest",
			Tags:        []string{"security", "dns", "adblock"},
			ComposeContent: `services:
  pihole:
    image: pihole/pihole:latest
    restart: unless-stopped
    ports:
      - "${PIHOLE_DNS_PORT:-53}:53/tcp"
      - "${PIHOLE_DNS_PORT:-53}:53/udp"
      - "${PIHOLE_WEB_PORT:-8080}:80"
    environment:
      TZ: ${TZ:-UTC}
      FTLCONF_webserver_api_password: ${PIHOLE_WEB_PASSWORD:?set PIHOLE_WEB_PASSWORD}
    volumes:
      - pihole-etc:/etc/pihole
      - pihole-dnsmasq:/etc/dnsmasq.d
volumes:
  pihole-etc:
  pihole-dnsmasq:
`,
			EnvContent: "PIHOLE_DNS_PORT=53\nPIHOLE_WEB_PORT=8080\nPIHOLE_WEB_PASSWORD=\nTZ=UTC\n",
			Notes:      "Set PIHOLE_WEB_PASSWORD before starting and avoid binding DNS ports on hosts already running a resolver.",
		},
		{
			ID:          "plex",
			Name:        "Plex Media Server",
			Description: "Personal media server for streaming libraries.",
			Category:    "media",
			Source:      "docker-hub-official",
			Image:       "plexinc/pms-docker:latest",
			Tags:        []string{"media", "video", "streaming"},
			ComposeContent: `services:
  plex:
    image: plexinc/pms-docker:latest
    restart: unless-stopped
    network_mode: host
    environment:
      TZ: ${TZ:-UTC}
      PLEX_CLAIM: ${PLEX_CLAIM:-}
      ADVERTISE_IP: ${PLEX_ADVERTISE_IP:-}
    volumes:
      - plex-config:/config
      - ./transcode:/transcode
      - ./media:/data:ro
volumes:
  plex-config:
`,
			EnvContent: "TZ=UTC\nPLEX_CLAIM=\nPLEX_ADVERTISE_IP=\n",
			Notes:      "Create media and transcode directories or edit the bind mounts. Host networking is recommended for Plex discovery.",
		},
		{
			ID:          "portainer-ce",
			Name:        "Portainer CE",
			Description: "Docker and Compose management UI.",
			Category:    "management",
			Source:      "official-docs",
			Image:       "portainer/portainer-ce:latest",
			Tags:        []string{"management", "docker", "containers"},
			ComposeContent: `services:
  portainer:
    image: portainer/portainer-ce:latest
    restart: unless-stopped
    ports:
      - "${PORTAINER_HTTPS_PORT:-9443}:9443"
      - "${PORTAINER_HTTP_PORT:-9000}:9000"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - portainer-data:/data
volumes:
  portainer-data:
`,
			EnvContent: "PORTAINER_HTTPS_PORT=9443\nPORTAINER_HTTP_PORT=9000\n",
			Notes:      "The Docker socket mount grants broad host control. Restrict network access to trusted operators.",
		},
		{
			ID:          "redpanda",
			Name:        "Redpanda",
			Description: "Kafka-compatible streaming platform without ZooKeeper.",
			Category:    "queue",
			Source:      "official-docs",
			Image:       "docker.redpanda.com/redpandadata/redpanda:latest",
			Tags:        []string{"queue", "streaming", "kafka-compatible"},
			ComposeContent: `services:
  redpanda:
    image: docker.redpanda.com/redpandadata/redpanda:latest
    restart: unless-stopped
    command:
      - redpanda
      - start
      - --overprovisioned
      - --smp=1
      - --memory=1G
      - --reserve-memory=0M
      - --node-id=0
      - --check=false
      - --kafka-addr=PLAINTEXT://0.0.0.0:9092
      - --advertise-kafka-addr=PLAINTEXT://${REDPANDA_ADVERTISED_HOST:-localhost}:${REDPANDA_KAFKA_PORT:-9092}
    ports:
      - "${REDPANDA_KAFKA_PORT:-9092}:9092"
      - "${REDPANDA_ADMIN_PORT:-9644}:9644"
    volumes:
      - redpanda-data:/var/lib/redpanda/data
volumes:
  redpanda-data:
`,
			EnvContent: "REDPANDA_KAFKA_PORT=9092\nREDPANDA_ADMIN_PORT=9644\nREDPANDA_ADVERTISED_HOST=localhost\n",
		},
		{
			ID:          "sftpgo",
			Name:        "SFTPGo",
			Description: "Managed SFTP, WebDAV, FTP, and object-storage gateway.",
			Category:    "files",
			Source:      "official-docs",
			Image:       "drakkan/sftpgo:latest",
			Tags:        []string{"files", "sftp", "webdav"},
			ComposeContent: `services:
  sftpgo:
    image: drakkan/sftpgo:latest
    restart: unless-stopped
    ports:
      - "${SFTPGO_HTTP_PORT:-8080}:8080"
      - "${SFTPGO_SFTP_PORT:-2022}:2022"
    volumes:
      - sftpgo-data:/srv/sftpgo
      - sftpgo-home:/var/lib/sftpgo
volumes:
  sftpgo-data:
  sftpgo-home:
`,
			EnvContent: "SFTPGO_HTTP_PORT=8080\nSFTPGO_SFTP_PORT=2022\n",
		},
		{
			ID:          "sonarqube",
			Name:        "SonarQube Community",
			Description: "Code quality and static analysis platform.",
			Category:    "devtools",
			Source:      "official-docs",
			Image:       "sonarqube:community",
			Tags:        []string{"devtools", "code-quality", "analysis"},
			ComposeContent: `services:
  sonarqube:
    image: sonarqube:community
    restart: unless-stopped
    ports:
      - "${SONARQUBE_PORT:-9000}:9000"
    environment:
      SONAR_JDBC_URL: jdbc:postgresql://db:5432/${POSTGRES_DB:-sonarqube}
      SONAR_JDBC_USERNAME: ${POSTGRES_USER:-sonarqube}
      SONAR_JDBC_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - sonarqube-data:/opt/sonarqube/data
      - sonarqube-extensions:/opt/sonarqube/extensions
      - sonarqube-logs:/opt/sonarqube/logs
    depends_on:
      - db
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-sonarqube}
      POSTGRES_USER: ${POSTGRES_USER:-sonarqube}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - sonarqube-db:/var/lib/postgresql/data
volumes:
  sonarqube-data:
  sonarqube-extensions:
  sonarqube-logs:
  sonarqube-db:
`,
			EnvContent: "SONARQUBE_PORT=9000\nPOSTGRES_DB=sonarqube\nPOSTGRES_USER=sonarqube\nPOSTGRES_PASSWORD=\n",
			Notes:      "Set POSTGRES_PASSWORD before starting. SonarQube may require host sysctl tuning for Elasticsearch.",
		},
		{
			ID:          "traefik-whoami",
			Name:        "Traefik Whoami",
			Description: "Tiny HTTP echo service for route and proxy testing.",
			Category:    "web",
			Source:      "official-docs",
			Image:       "traefik/whoami:latest",
			Tags:        []string{"web", "testing", "http"},
			ComposeContent: `services:
  whoami:
    image: traefik/whoami:latest
    restart: unless-stopped
    ports:
      - "${WHOAMI_PORT:-8080}:80"
`,
			EnvContent: "WHOAMI_PORT=8080\n",
		},
		{
			ID:          "trivy-server",
			Name:        "Trivy Server",
			Description: "Vulnerability scanner server for container and filesystem scans.",
			Category:    "security",
			Source:      "official-docs",
			Image:       "aquasec/trivy:latest",
			Tags:        []string{"security", "scanner", "vulnerabilities"},
			ComposeContent: `services:
  trivy:
    image: aquasec/trivy:latest
    restart: unless-stopped
    command: ["server", "--listen", "0.0.0.0:4954"]
    ports:
      - "${TRIVY_PORT:-4954}:4954"
    volumes:
      - trivy-cache:/root/.cache
volumes:
  trivy-cache:
`,
			EnvContent: "TRIVY_PORT=4954\n",
			Notes:      "Restrict access to trusted clients; the server accepts scan requests over HTTP.",
		},
		{
			ID:          "watchtower",
			Name:        "Watchtower",
			Description: "Automated container image updater.",
			Category:    "automation",
			Source:      "official-docs",
			Image:       "containrrr/watchtower:latest",
			Tags:        []string{"automation", "updates", "docker"},
			ComposeContent: `services:
  watchtower:
    image: containrrr/watchtower:latest
    restart: unless-stopped
    command: ["--schedule", "${WATCHTOWER_SCHEDULE:-0 0 4 * * *}", "--cleanup"]
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
`,
			EnvContent: "WATCHTOWER_SCHEDULE=0 0 4 * * *\n",
			Notes:      "Automated updates can restart managed containers. Use labels or scope arguments if only selected stacks should update.",
		},
		{
			ID:          "wikijs",
			Name:        "Wiki.js",
			Description: "Modern wiki and knowledge base backed by PostgreSQL.",
			Category:    "docs",
			Source:      "official-docs",
			Image:       "ghcr.io/requarks/wiki:2",
			Tags:        []string{"docs", "wiki", "knowledge-base"},
			ComposeContent: `services:
  wiki:
    image: ghcr.io/requarks/wiki:2
    restart: unless-stopped
    ports:
      - "${WIKIJS_PORT:-8080}:3000"
    environment:
      DB_TYPE: postgres
      DB_HOST: db
      DB_PORT: 5432
      DB_USER: ${POSTGRES_USER:-wikijs}
      DB_PASS: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      DB_NAME: ${POSTGRES_DB:-wiki}
    depends_on:
      - db
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-wiki}
      POSTGRES_USER: ${POSTGRES_USER:-wikijs}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - wikijs-db:/var/lib/postgresql/data
volumes:
  wikijs-db:
`,
			EnvContent: "WIKIJS_PORT=8080\nPOSTGRES_DB=wiki\nPOSTGRES_USER=wikijs\nPOSTGRES_PASSWORD=\n",
			Notes:      "Set POSTGRES_PASSWORD before starting.",
		},
	}
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

func linuxserverMediaTemplates() []StackTemplate {
	return []StackTemplate{
		{
			ID:          "radarr",
			Name:        "Radarr",
			Description: "Movie management and automated downloading.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/radarr:latest",
			Tags:        []string{"media", "movies", "automation", "pvr"},
			ComposeContent: `services:
  radarr:
    image: lscr.io/linuxserver/radarr:latest
    restart: unless-stopped
    ports:
      - "${RADARR_PORT:-7878}:7878"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - radarr-config:/config
      - radarr-downloads:/downloads
volumes:
  radarr-config:
  radarr-downloads:
`,
			EnvContent: "RADARR_PORT=7878\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Access the web UI at the configured port. Connect to a download client and indexer after first login.",
		},
		{
			ID:          "sonarr",
			Name:        "Sonarr",
			Description: "TV show management and automated downloading.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/sonarr:latest",
			Tags:        []string{"media", "tv", "automation", "pvr"},
			ComposeContent: `services:
  sonarr:
    image: lscr.io/linuxserver/sonarr:latest
    restart: unless-stopped
    ports:
      - "${SONARR_PORT:-8989}:8989"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - sonarr-config:/config
      - sonarr-downloads:/downloads
volumes:
  sonarr-config:
  sonarr-downloads:
`,
			EnvContent: "SONARR_PORT=8989\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Access the web UI at the configured port. Connect to a download client and indexer after first login.",
		},
		{
			ID:          "lidarr",
			Name:        "Lidarr",
			Description: "Music collection management and automated downloading.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/lidarr:latest",
			Tags:        []string{"media", "music", "automation", "pvr"},
			ComposeContent: `services:
  lidarr:
    image: lscr.io/linuxserver/lidarr:latest
    restart: unless-stopped
    ports:
      - "${LIDARR_PORT:-8686}:8686"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - lidarr-config:/config
      - lidarr-downloads:/downloads
volumes:
  lidarr-config:
  lidarr-downloads:
`,
			EnvContent: "LIDARR_PORT=8686\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Access the web UI at the configured port. Connect to a download client and indexer after first login.",
		},
		{
			ID:          "prowlarr",
			Name:        "Prowlarr",
			Description: "Indexer manager and proxy for Sonarr, Radarr, Lidarr, and Readarr.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/prowlarr:latest",
			Tags:        []string{"media", "indexer", "automation", "pvr"},
			ComposeContent: `services:
  prowlarr:
    image: lscr.io/linuxserver/prowlarr:latest
    restart: unless-stopped
    ports:
      - "${PROWLARR_PORT:-9696}:9696"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - prowlarr-config:/config
volumes:
  prowlarr-config:
`,
			EnvContent: "PROWLARR_PORT=9696\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Central indexer manager. Add indexers here, then connect Prowlarr to Sonarr/Radarr/Lidarr for unified search.",
		},
		{
			ID:          "bazarr",
			Name:        "Bazarr",
			Description: "Subtitle management for Sonarr and Radarr.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/bazarr:latest",
			Tags:        []string{"media", "subtitles", "automation"},
			ComposeContent: `services:
  bazarr:
    image: lscr.io/linuxserver/bazarr:latest
    restart: unless-stopped
    ports:
      - "${BAZARR_PORT:-6767}:6767"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - bazarr-config:/config
volumes:
  bazarr-config:
`,
			EnvContent: "BAZARR_PORT=6767\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Connect to Sonarr and Radarr after first login to enable automatic subtitle downloads.",
		},
		{
			ID:          "nzbget",
			Name:        "NZBGet",
			Description: "Efficient Usenet downloader.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/nzbget:latest",
			Tags:        []string{"media", "usenet", "downloader"},
			ComposeContent: `services:
  nzbget:
    image: lscr.io/linuxserver/nzbget:latest
    restart: unless-stopped
    ports:
      - "${NZBGET_PORT:-6789}:6789"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - nzbget-config:/config
      - nzbget-downloads:/downloads
volumes:
  nzbget-config:
  nzbget-downloads:
`,
			EnvContent: "NZBGET_PORT=6789\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Default login: nzbget / tegbzn6789. Change the password after first login.",
		},
		{
			ID:          "sabnzbd",
			Name:        "SABnzbd",
			Description: "Free and open-source Usenet downloader.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/sabnzbd:latest",
			Tags:        []string{"media", "usenet", "downloader"},
			ComposeContent: `services:
  sabnzbd:
    image: lscr.io/linuxserver/sabnzbd:latest
    restart: unless-stopped
    ports:
      - "${SABNZBD_PORT:-8080}:8080"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - sabnzbd-config:/config
      - sabnzbd-downloads:/downloads
volumes:
  sabnzbd-config:
  sabnzbd-downloads:
`,
			EnvContent: "SABNZBD_PORT=8080\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Run the setup wizard on first access to configure Usenet servers.",
		},
		{
			ID:          "qbittorrent",
			Name:        "qBittorrent",
			Description: "BitTorrent client with web UI.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/qbittorrent:latest",
			Tags:        []string{"media", "torrent", "downloader"},
			ComposeContent: `services:
  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent:latest
    restart: unless-stopped
    ports:
      - "${QBITTORRENT_WEBUI_PORT:-8080}:8080"
      - "6881:6881"
      - "6881:6881/udp"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
      - WEBUI_PORT=8080
    volumes:
      - qbittorrent-config:/config
      - qbittorrent-downloads:/downloads
volumes:
  qbittorrent-config:
  qbittorrent-downloads:
`,
			EnvContent: "QBITTORRENT_WEBUI_PORT=8080\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Default login: admin / adminadmin. Change password immediately. Port 6881 is the default BitTorrent listen port.",
		},
		{
			ID:          "transmission",
			Name:        "Transmission",
			Description: "Lightweight BitTorrent client with web UI.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/transmission:latest",
			Tags:        []string{"media", "torrent", "downloader"},
			ComposeContent: `services:
  transmission:
    image: lscr.io/linuxserver/transmission:latest
    restart: unless-stopped
    ports:
      - "${TRANSMISSION_PORT:-9091}:9091"
      - "51413:51413"
      - "51413:51413/udp"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - transmission-config:/config
      - transmission-downloads:/downloads
volumes:
  transmission-config:
  transmission-downloads:
`,
			EnvContent: "TRANSMISSION_PORT=9091\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Web UI available at the configured port. No default password unless USER and PASS env vars are set.",
		},
		{
			ID:          "overseerr",
			Name:        "Overseerr",
			Description: "Media request and discovery management for Plex.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/overseerr:latest",
			Tags:        []string{"media", "requests", "plex"},
			ComposeContent: `services:
  overseerr:
    image: lscr.io/linuxserver/overseerr:latest
    restart: unless-stopped
    ports:
      - "${OVERSEERR_PORT:-5055}:5055"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - overseerr-config:/config
volumes:
  overseerr-config:
`,
			EnvContent: "OVERSEERR_PORT=5055\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Connect to your Plex server during initial setup. Users can request movies and TV shows through the web UI.",
		},
		{
			ID:          "tautulli",
			Name:        "Tautulli",
			Description: "Monitoring and tracking for Plex Media Server.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/tautulli:latest",
			Tags:        []string{"media", "plex", "monitoring", "statistics"},
			ComposeContent: `services:
  tautulli:
    image: lscr.io/linuxserver/tautulli:latest
    restart: unless-stopped
    ports:
      - "${TAUTULLI_PORT:-8181}:8181"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - tautulli-config:/config
volumes:
  tautulli-config:
`,
			EnvContent: "TAUTULLI_PORT=8181\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Connect to your Plex server during setup to enable activity monitoring, history, and statistics.",
		},
		{
			ID:          "readarr",
			Name:        "Readarr",
			Description: "Book and audiobook management and automation.",
			Category:    "media",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/readarr:develop",
			Tags:        []string{"media", "books", "audiobooks", "automation", "pvr"},
			ComposeContent: `services:
  readarr:
    image: lscr.io/linuxserver/readarr:develop
    restart: unless-stopped
    ports:
      - "${READARR_PORT:-8787}:8787"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - readarr-config:/config
      - readarr-downloads:/downloads
volumes:
  readarr-config:
  readarr-downloads:
`,
			EnvContent: "READARR_PORT=8787\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Uses the develop tag because Readarr has not had a stable release yet. Connect to a download client and indexer after first login.",
		},
	}
}

func gamingTemplates() []StackTemplate {
	return []StackTemplate{
		{
			ID:          "emulatorjs",
			Name:        "EmulatorJS",
			Description: "In-browser retro game emulation with ROM management.",
			Category:    "gaming",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/emulatorjs:latest",
			Tags:        []string{"gaming", "emulation", "retro", "browser"},
			ComposeContent: `services:
  emulatorjs:
    image: lscr.io/linuxserver/emulatorjs:latest
    restart: unless-stopped
    ports:
      - "${EMULATORJS_PORT:-3000}:3000"
      - "${EMULATORJS_MGMT_PORT:-3001}:3001"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - emulatorjs-config:/config
      - emulatorjs-data:/data
volumes:
  emulatorjs-config:
  emulatorjs-data:
`,
			EnvContent: "EMULATORJS_PORT=3000\nEMULATORJS_MGMT_PORT=3001\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Port 3000 is the player frontend, port 3001 is the ROM management backend. Upload ROMs through the management UI.",
		},
		{
			ID:          "sunshine",
			Name:        "Sunshine",
			Description: "Self-hosted game streaming server compatible with Moonlight clients.",
			Category:    "gaming",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/sunshine:latest",
			Tags:        []string{"gaming", "streaming", "moonlight", "gpu"},
			ComposeContent: `services:
  sunshine:
    image: lscr.io/linuxserver/sunshine:latest
    restart: unless-stopped
    ports:
      - "${SUNSHINE_PORT:-47990}:47990"
      - "47984-47989:47984-47989"
      - "48010:48010"
      - "47998-48000:47998-48000/udp"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - sunshine-config:/config
volumes:
  sunshine-config:
`,
			EnvContent: "SUNSHINE_PORT=47990\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Access the web UI on port 47990 to configure. Pair with a Moonlight client on another device to stream games. GPU passthrough recommended for best performance.",
		},
	}
}

func remoteDesktopTemplates() []StackTemplate {
	return []StackTemplate{
		{
			ID:          "webtop",
			Name:        "Webtop",
			Description: "Full Linux desktop accessible from the browser (Ubuntu XFCE).",
			Category:    "remote",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/webtop:ubuntu-xfce",
			Tags:        []string{"remote", "desktop", "linux", "browser", "vnc"},
			ComposeContent: `services:
  webtop:
    image: lscr.io/linuxserver/webtop:ubuntu-xfce
    restart: unless-stopped
    ports:
      - "${WEBTOP_PORT:-3000}:3000"
      - "${WEBTOP_HTTPS_PORT:-3001}:3001"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - webtop-config:/config
    shm_size: "1gb"
volumes:
  webtop-config:
`,
			EnvContent: "WEBTOP_PORT=3000\nWEBTOP_HTTPS_PORT=3001\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Access the desktop at the configured port. Other desktop variants available: alpine-kde, fedora-xfce, arch-xfce, etc. Change the image tag to switch.",
		},
		{
			ID:          "firefox",
			Name:        "Firefox Browser",
			Description: "Firefox web browser running in Docker, accessible from the browser.",
			Category:    "remote",
			Source:      "linuxserver",
			Image:       "lscr.io/linuxserver/firefox:latest",
			Tags:        []string{"remote", "browser", "firefox", "vnc"},
			ComposeContent: `services:
  firefox:
    image: lscr.io/linuxserver/firefox:latest
    restart: unless-stopped
    ports:
      - "${FIREFOX_PORT:-3000}:3000"
      - "${FIREFOX_HTTPS_PORT:-3001}:3001"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - firefox-config:/config
    shm_size: "1gb"
volumes:
  firefox-config:
`,
			EnvContent: "FIREFOX_PORT=3000\nFIREFOX_HTTPS_PORT=3001\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Isolated browser session in Docker. Useful for secure browsing, testing, or accessing internal services from a remote host.",
		},
	}
}
