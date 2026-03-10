# Linux (Ubuntu) Deployment Guide

## 🚀 Environment Variables Setup

### Option 1: Systemd Service (Önerilen - Production)

Her servis için systemd unit file oluştur:

#### 1. Acquirer Service

```bash
sudo nano /etc/systemd/system/acquirer.service
```

```ini
[Unit]
Description=Payment Acquirer Service
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/payment-lab
ExecStart=/opt/payment-lab/acquirer

# Environment Variables
Environment="ACQUIRER_PORT=8081"
Environment="ISSUER_ADDR=localhost:5001"
Environment="OTEL_SERVICE_NAME=acquirer"
Environment="OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp"
Environment="OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"
Environment="OTEL_RESOURCE_ATTRIBUTES=service.namespace=paymentlab,deployment.environment=production"
Environment="OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic MTU1Mjc1OTpnbGNfZXlKdklqb2lNVFk1TXpVMU5pSXNJbTRpT2lKbmNtRndhR0Z1WVMxdmRHVnNMV1JwY21WamRDMTBiMnRsYmlJc0ltc2lPaUpsT0RKUVJqQTFPVTh6VkRsak9EUlJNa0pIVmtWd1ZYQWlMQ0p0SWpwN0luSWlPaUp3Y205a0xXVjFMWGRsYzNRdE1pSjlmUT09"

Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

#### 2. Issuer Service

```bash
sudo nano /etc/systemd/system/issuer.service
```

```ini
[Unit]
Description=Payment Issuer Service
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/payment-lab
ExecStart=/opt/payment-lab/issuer

# Environment Variables
Environment="ISSUER_ADDR=0.0.0.0:5001"
Environment="OTEL_SERVICE_NAME=issuer"
Environment="OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp"
Environment="OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"
Environment="OTEL_RESOURCE_ATTRIBUTES=service.namespace=paymentlab,deployment.environment=production"
Environment="OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic MTU1Mjc1OTpnbGNfZXlKdklqb2lNVFk1TXpVMU5pSXNJbTRpT2lKbmNtRndhR0Z1WVMxdmRHVnNMV1JwY21WamRDMTBiMnRsYmlJc0ltc2lPaUpsT0RKUVJqQTFPVTh6VkRsak9EUlJNa0pIVmtWd1ZYQWlMQ0p0SWpwN0luSWlPaUp3Y205a0xXVjFMWGRsYzNRdE1pSjlmUT09"

Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

#### 3. Servisleri Aktifleştir

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable services (boot'ta otomatik başlasın)
sudo systemctl enable acquirer
sudo systemctl enable issuer

# Start services
sudo systemctl start acquirer
sudo systemctl start issuer

# Check status
sudo systemctl status acquirer
sudo systemctl status issuer

# View logs
sudo journalctl -u acquirer -f
sudo journalctl -u issuer -f
```

---

### Option 2: /etc/environment (Global - Basit)

Tüm kullanıcılar için global environment variables:

```bash
sudo nano /etc/environment
```

Ekle:

```bash
ACQUIRER_PORT=8081
ISSUER_ADDR=localhost:5001
OTEL_SERVICE_NAME=acquirer
OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_RESOURCE_ATTRIBUTES=service.namespace=paymentlab,deployment.environment=production
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic MTU1Mjc1OTpnbGNfZXlKdklqb2lNVFk1TXpVMU5pSXNJbTRpT2lKbmNtRndhR0Z1WVMxdmRHVnNMV1JwY21WamRDMTBiMnRsYmlJc0ltc2lPaUpsT0RKUVJqQTFPVTh6VkRsak9EUlJNa0pIVmtWd1ZYQWlMQ0p0SWpwN0luSWlPaUp3Y205a0xXVjFMWGRsYzNRdE1pSjlmUT09
```

Logout/login veya reboot gerekli.

---

### Option 3: ~/.bashrc veya ~/.profile (User-specific)

Sadece belirli bir kullanıcı için:

```bash
nano ~/.bashrc
```

En alta ekle:

```bash
# Payment Lab Environment Variables
export ACQUIRER_PORT=8081
export ISSUER_ADDR=localhost:5001
export OTEL_SERVICE_NAME=acquirer
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_RESOURCE_ATTRIBUTES=service.namespace=paymentlab,deployment.environment=production
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic MTU1Mjc1OTpnbGNfZXlKdklqb2lNVFk1TXpVMU5pSXNJbTRpT2lKbmNtRndhR0Z1WVMxdmRHVnNMV1JwY21WamRDMTBiMnRsYmlJc0ltc2lPaUpsT0RKUVJqQTFPVTh6VkRsak9EUlJNa0pIVmtWd1ZYQWlMQ0p0SWpwN0luSWlPaUp3Y205a0xXVjFMWGRsYzNRdE1pSjlmUT09"
```

Aktifleştir:

```bash
source ~/.bashrc
```

---

### Option 4: .env File + systemd EnvironmentFile (En Temiz)

#### 1. .env dosyası oluştur

```bash
sudo mkdir -p /opt/payment-lab/config
sudo nano /opt/payment-lab/config/acquirer.env
```

```bash
ACQUIRER_PORT=8081
ISSUER_ADDR=localhost:5001
OTEL_SERVICE_NAME=acquirer
OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_RESOURCE_ATTRIBUTES=service.namespace=paymentlab,deployment.environment=production
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic MTU1Mjc1OTpnbGNfZXlKdklqb2lNVFk1TXpVMU5pSXNJbTRpT2lKbmNtRndhR0Z1WVMxdmRHVnNMV1JwY21WamRDMTBiMnRsYmlJc0ltc2lPaUpsT0RKUVJqQTFPVTh6VkRsak9EUlJNa0pIVmtWd1ZYQWlMQ0p0SWpwN0luSWlPaUp3Y205a0xXVjFMWGRsYzNRdE1pSjlmUT09
```

```bash
sudo nano /opt/payment-lab/config/issuer.env
```

```bash
ISSUER_ADDR=0.0.0.0:5001
OTEL_SERVICE_NAME=issuer
OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_RESOURCE_ATTRIBUTES=service.namespace=paymentlab,deployment.environment=production
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic MTU1Mjc1OTpnbGNfZXlKdklqb2lNVFk1TXpVMU5pSXNJbTRpT2lKbmNtRndhR0Z1WVMxdmRHVnNMV1JwY21WamRDMTBiMnRsYmlJc0ltc2lPaUpsT0RKUVJqQTFPVTh6VkRsak9EUlJNa0pIVmtWd1ZYQWlMQ0p0SWpwN0luSWlPaUp3Y205a0xXVjFMWGRsYzNRdE1pSjlmUT09
```

#### 2. Systemd service'de kullan

```ini
[Service]
EnvironmentFile=/opt/payment-lab/config/acquirer.env
ExecStart=/opt/payment-lab/acquirer
```

---

## 📦 Deployment Adımları

### 1. Binary'leri Derle (Windows'ta)

```powershell
# Linux için cross-compile
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o acquirer ./cmd/acquirer
go build -o issuer ./cmd/issuer
```

### 2. Ubuntu'ya Aktar

```bash
# SCP ile
scp acquirer issuer ubuntu@your-server:/opt/payment-lab/

# Veya rsync ile
rsync -avz acquirer issuer ubuntu@your-server:/opt/payment-lab/
```

### 3. Executable Yap

```bash
sudo chmod +x /opt/payment-lab/acquirer
sudo chmod +x /opt/payment-lab/issuer
```

### 4. Firewall Ayarları (Gerekirse)

```bash
# Port 8081 (Acquirer HTTP)
sudo ufw allow 8081/tcp

# Port 5001 (Issuer TCP) - Sadece localhost'tan erişim istiyorsan gerek yok
# sudo ufw allow 5001/tcp
```

---

## 🔍 Monitoring & Troubleshooting

### Logs

```bash
# Systemd logs
sudo journalctl -u acquirer -f
sudo journalctl -u issuer -f

# Son 100 satır
sudo journalctl -u acquirer -n 100

# Belirli tarih aralığı
sudo journalctl -u acquirer --since "2026-03-10 00:00:00" --until "2026-03-10 23:59:59"
```

### Process Kontrolü

```bash
# Process var mı?
ps aux | grep acquirer
ps aux | grep issuer

# Port dinliyor mu?
sudo netstat -tlnp | grep 8081
sudo netstat -tlnp | grep 5001

# Veya ss ile
sudo ss -tlnp | grep 8081
```

### Health Check

```bash
# Acquirer health
curl http://localhost:8081/health

# Transaction test
curl -X POST http://localhost:8081/v1/transaction \
  -H "Content-Type: application/json" \
  -d '{
    "MTI": "0100",
    "PAN": "4242424242424242",
    "ProcCode": "000000",
    "AmountTrn": "000000001000",
    "LocalTime": "123456",
    "LocalDate": "0101",
    "STAN": "123456",
    "TerminalID": "TERM0001",
    "MerchantID": "MERC00000000001"
  }'
```

---

## 🔐 Güvenlik Notları

1. **Grafana Cloud Token'ı Güvende Tut**
   - `.env` dosyalarına `chmod 600` ver
   - Sadece servis kullanıcısı okuyabilsin

2. **Issuer Sadece Localhost'tan Erişilebilir**
   - `ISSUER_ADDR=localhost:5001` (dışarıya açma)
   - Eğer farklı sunucularda çalışacaksa, firewall kuralları ekle

3. **Acquirer Public Endpoint**
   - `ACQUIRER_PORT=8081` - Nginx reverse proxy arkasında çalıştır
   - Rate limiting ekle
   - HTTPS kullan (Let's Encrypt)

---

## 📊 Performans Tuning

### Systemd Service Limits

```ini
[Service]
LimitNOFILE=65536
LimitNPROC=4096
```

### Kernel Parameters (100K RPS için)

```bash
sudo nano /etc/sysctl.conf
```

```bash
# TCP tuning
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30

# File descriptors
fs.file-max = 2097152
```

Aktifleştir:

```bash
sudo sysctl -p
```

---

## ✅ Özet: En İyi Yöntem

**Production için önerilen:** **Option 4** (systemd + EnvironmentFile)

1. ✅ Environment variables dosyada, kolay güncelleme
2. ✅ Systemd ile otomatik restart
3. ✅ Log management (journalctl)
4. ✅ Güvenli (file permissions)
5. ✅ Boot'ta otomatik başlatma

**Development için:** **Option 3** (~/.bashrc)
