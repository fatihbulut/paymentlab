# Server Setup Guide

This guide covers the one-time setup required on both issuer and acquirer servers for Docker Compose deployment.

## Prerequisites

Both servers need:
- Ubuntu 20.04+ or similar Linux distribution
- SSH access with sudo privileges
- Network connectivity between servers

---

## 1. Install Docker & Docker Compose

Run on **both issuer and acquirer servers**:

```bash
# Update package index
sudo apt update

# Install dependencies
sudo apt install -y ca-certificates curl gnupg lsb-release

# Add Docker's official GPG key
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Set up Docker repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker Engine & Docker Compose
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Add your user to docker group (to run docker without sudo)
sudo usermod -aG docker $USER

# Log out and back in for group changes to take effect
exit
```

After logging back in, verify installation:

```bash
docker --version
docker compose version
```

---

## 2. Configure Issuer Server

SSH into the issuer server:

```bash
ssh ubuntu@<issuer-ip>
```

### Create environment file:

```bash
# Copy example env file
cp .env.issuer.example ~/.env

# Edit with your values
nano ~/.env
```

Set these values in `~/.env`:
```bash
POSTGRES_PASSWORD=<strong-password-here>
ISSUER_IMAGE=ghcr.io/fatihbulut/iso-parser-service-issuer:latest
```

### Configure firewall (allow acquirer to connect):

```bash
# Allow PostgreSQL port from acquirer
sudo ufw allow from <acquirer-ip> to any port 5432

# Allow Issuer TCP service from acquirer
sudo ufw allow from <acquirer-ip> to any port 5001

# Reload firewall
sudo ufw reload
```

---

## 3. Configure Acquirer Server

SSH into the acquirer server:

```bash
ssh ubuntu@<acquirer-ip>
```

### Create environment file:

```bash
# Copy example env file
cp .env.acquirer.example ~/.env

# Edit with your values
nano ~/.env
```

Set these values in `~/.env`:
```bash
ISSUER_ADDR=<issuer-ip>:5001
POSTGRES_HOST=<issuer-ip>
POSTGRES_PASSWORD=<same-password-as-issuer>
ACQUIRER_PORT=8081
ACQUIRER_IMAGE=ghcr.io/fatihbulut/iso-parser-service-acquirer:latest
```

### Configure firewall (allow HTTP traffic):

```bash
# Allow HTTP port 8081
sudo ufw allow 8081/tcp

# Reload firewall
sudo ufw reload
```

---

## 4. GitHub Container Registry Authentication

Both servers need to authenticate with ghcr.io to pull private images.

### On each server:

```bash
# Login to GitHub Container Registry
echo "<YOUR_GITHUB_TOKEN>" | docker login ghcr.io -u <YOUR_GITHUB_USERNAME> --password-stdin
```

**Note:** Create a GitHub Personal Access Token with `read:packages` scope at:
https://github.com/settings/tokens

---

## 5. Test Network Connectivity

### From acquirer server, test connection to issuer:

```bash
# Test PostgreSQL port
nc -zv <issuer-ip> 5432

# Test Issuer TCP service port
nc -zv <issuer-ip> 5001
```

Both should return `succeeded` or `open`.

---

## 6. Stop Old Services (if running)

If you previously deployed with systemd, stop those services:

### On issuer:
```bash
sudo systemctl stop issuer.service
sudo systemctl disable issuer.service
```

### On acquirer:
```bash
sudo systemctl stop acquirer.service
sudo systemctl disable acquirer.service
```

---

## 7. Initial Deployment

GitHub Actions will handle deployment automatically on push to `staging` branch.

But you can also manually deploy:

### On issuer:
```bash
cd ~
docker compose -f docker-compose.issuer.yml pull
docker compose -f docker-compose.issuer.yml up -d
```

### On acquirer:
```bash
cd ~
docker compose -f docker-compose.acquirer.yml pull
docker compose -f docker-compose.acquirer.yml up -d
```

---

## 8. Verify Deployment

### Check running containers:

```bash
# On issuer
docker compose -f ~/docker-compose.issuer.yml ps

# On acquirer
docker compose -f ~/docker-compose.acquirer.yml ps
```

### Check logs:

```bash
# Issuer logs
docker compose -f ~/docker-compose.issuer.yml logs -f issuer
docker compose -f ~/docker-compose.issuer.yml logs -f postgres

# Acquirer logs
docker compose -f ~/docker-compose.acquirer.yml logs -f acquirer
```

### Test the application:

```bash
# From acquirer server
curl http://localhost:8081/health
```

Should return: `{"status":"ok"}`

---

## Troubleshooting

### Container won't start:
```bash
# Check logs
docker compose -f ~/docker-compose.issuer.yml logs

# Restart services
docker compose -f ~/docker-compose.issuer.yml restart
```

### Network issues:
```bash
# Check firewall rules
sudo ufw status verbose

# Test connectivity
nc -zv <target-ip> <port>
```

### Permission issues:
```bash
# Ensure user is in docker group
groups $USER

# If not, add and re-login
sudo usermod -aG docker $USER
exit
```

---

## Maintenance Commands

### View logs:
```bash
docker compose -f ~/docker-compose.issuer.yml logs -f
```

### Restart services:
```bash
docker compose -f ~/docker-compose.issuer.yml restart
```

### Stop services:
```bash
docker compose -f ~/docker-compose.issuer.yml down
```

### Update to latest images:
```bash
docker compose -f ~/docker-compose.issuer.yml pull
docker compose -f ~/docker-compose.issuer.yml up -d
```

### Clean up old images:
```bash
docker image prune -f
```
