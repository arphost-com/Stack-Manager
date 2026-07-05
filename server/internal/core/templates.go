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
