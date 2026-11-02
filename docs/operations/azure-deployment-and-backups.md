# Azure Deployment, Persistence, Backups & Cost-Control Runbook

This operations guide covers provisioning, TLS setup, persistent SQLite disk management, online backup & disaster recovery, and student budget cost-control for **ME Lab** on Microsoft Azure.

---

## 1. Production Topology Summary

- **Compute**: 1x Ubuntu 24.04 LTS Linux VM (`Standard_B2s` default: 2 vCPU, 4 GiB RAM) with **System-Assigned Managed Identity**.
- **Ingress & TLS**: Azure Public IP (`<dnsLabelPrefix>.<region>.cloudapp.azure.com`) + Network Security Group -> Host **Nginx** (`deployments/azure/nginx-tls.conf`) terminating Let's Encrypt TLS 1.2/1.3 -> Docker Compose (`deployments/azure/compose.prod.yaml`) bound strictly to `127.0.0.1:8080` (`api`) and `127.0.0.1:8081` (`web`).
- **Persistent SQLite Disk**: 32 GiB `StandardSSD_LRS` Azure Managed Data Disk (`LUN 0`, `caching: None`) formatted as `ext4` and mounted at `/mnt/melab-data`, storing `/mnt/melab-data/me-lab.db` in WAL mode.
- **Off-Host Backups**: Azure Storage Account (`Standard_LRS`, public blob access disabled, TLS 1.2+) with private container `sqlite-backups`. The VM's System-Assigned Managed Identity holds the **Storage Blob Data Contributor** RBAC role (`ba92f5b4-2d11-453d-a403-e96b0029c9fe`) — no storage account keys or SAS tokens are stored on the VM.

---

## 2. Provisioning Infrastructure with Azure Bicep

### Prerequisites
- Azure CLI (`az >= 2.60`) with Bicep (`az bicep version`)
- An SSH Ed25519 keypair (`~/.ssh/id_ed25519.pub`)
- Your workstation's public IPv4 CIDR for SSH lockdown (`MY_IP="$(curl -fsS https://checkip.amazonaws.com)/32"`)

### Deploy Resource Group & Bicep Template

```bash
export AZURE_RG="rg-melab-prod"
export AZURE_LOCATION="eastus2"
export DNS_LABEL="melab-ae2-prod"
export MY_SSH_CIDR="$(curl -fsS https://checkip.amazonaws.com | tr -d '\n')/32"

# 1. Create resource group
az group create \
  --name "$AZURE_RG" \
  --location "$AZURE_LOCATION"

# 2. Validate & deploy Bicep infrastructure
az deployment group create \
  --resource-group "$AZURE_RG" \
  --template-file deployments/azure/main.bicep \
  --parameters \
    environmentName="prod" \
    vmSize="Standard_B2s" \
    adminUsername="azureuser" \
    sshPublicKey="$(cat ~/.ssh/id_ed25519.pub)" \
    sshAllowedCidr="$MY_SSH_CIDR" \
    dnsLabelPrefix="$DNS_LABEL"

# 3. Inspect deployment outputs (FQDN, public IP, storage account name)
az deployment group show \
  --resource-group "$AZURE_RG" \
  --name main \
  --query properties.outputs \
  --output table
```

---

## 3. Network Security Group (NSG) Rules

`deployments/azure/main.bicep` enforces least-privilege inbound access on the subnet NSG:

| Priority | Name | Direction | Protocol | Port | Source | Destination | Action | Purpose |
| ---: | --- | --- | --- | ---: | --- | --- | --- | --- |
| `100` | `Allow-HTTP-ACME` | Inbound | TCP | `80` | `Internet` (`*`) | `*` | Allow | Let's Encrypt HTTP-01 challenge & `301` redirect to HTTPS |
| `110` | `Allow-HTTPS` | Inbound | TCP | `443` | `Internet` (`*`) | `*` | Allow | Public TLS 1.2/1.3 ingress to Host Nginx |
| `120` | `Allow-SSH-Restricted` | Inbound | TCP | `22` | `sshAllowedCidr` | `*` | Allow | Operator SSH access from allowlisted IP/CIDR only |
| `65500` | `DenyAllInBound` | Inbound | `*` | `*` | `*` | `*` | Deny | Azure default deny-all catch-all (ports `8080`/`8081` unreachable externally) |

---

## 4. Persistent Managed Disk Mount (`/mnt/melab-data`)

`deployments/azure/cloud-init.yaml` automatically initializes `LUN 0` (`/dev/disk/azure/scsi1/lun0`) on first boot. To verify or perform manual setup on an existing VM:

```bash
# Verify block device, filesystem UUID, and fstab mount
lsblk -f /dev/disk/azure/scsi1/lun0
df -h /mnt/melab-data

# Ensure ownership matches the non-root melab container user (UID/GID 10001)
sudo chown -R 10001:10001 /mnt/melab-data
sudo chmod 0750 /mnt/melab-data
```

Because `/mnt/melab-data` resides on an independent Azure Managed Disk with `caching: None`:
1. SQLite `WAL` commits (`fsync`) flush directly to durable Azure SSD storage.
2. Container upgrades (`docker compose pull && docker compose up -d`) and OS image redeployments never wipe `me-lab.db`.

---

## 5. Let's Encrypt TLS & Application Bootstrap

SSH into the provisioned VM and configure `/opt/me-lab/.env` plus Let's Encrypt certificates:

```bash
export FQDN="${DNS_LABEL}.${AZURE_LOCATION}.cloudapp.azure.com"
export STORAGE_ACCT="$(az deployment group show -g "$AZURE_RG" -n main --query properties.outputs.storageAccountName.value -o tsv)"

ssh "azureuser@${FQDN}"
```

On the VM:

```bash
# 1. Clone repository configuration & scripts into /opt/me-lab
sudo git clone https://github.com/felipinho/me-lab.git /opt/me-lab
cd /opt/me-lab

# 2. Create production .env (0600 root-owned)
sudo tee /opt/me-lab/.env >/dev/null <<EOF
PORT=8080
DATABASE_URL=/mnt/melab-data/me-lab.db
JWT_SECRET=$(openssl rand -hex 32)
JWT_EXPIRY_HOURS=24
LOG_LEVEL=info
CORS_ALLOWED_ORIGINS=https://<YOUR_FQDN>
GITHUB_REPOSITORY=felipinho/me-lab
AZURE_STORAGE_ACCOUNT=<YOUR_STORAGE_ACCOUNT>
AZURE_STORAGE_CONTAINER=sqlite-backups
AZURE_USE_MANAGED_IDENTITY=true
RETENTION_DAYS=30
EOF
sudo chmod 0600 /opt/me-lab/.env

# 3. Install Nginx site config & request Let's Encrypt certificate
sudo mkdir -p /var/www/certbot
sudo sed "s/melab.example.com/<YOUR_FQDN>/g" /opt/me-lab/deployments/azure/nginx-tls.conf \
  | sudo tee /etc/nginx/sites-available/me-lab.conf >/dev/null
sudo rm -f /etc/nginx/sites-enabled/default

# Obtain initial certificate via Certbot standalone/webroot, then enable full TLS vhost
sudo certbot certonly --nginx -d "<YOUR_FQDN>" --non-interactive --agree-tos -m "ops@example.com"
sudo ln -sf /etc/nginx/sites-available/me-lab.conf /etc/nginx/sites-enabled/me-lab.conf
sudo nginx -t && sudo systemctl reload nginx

# 4. Verify automatic certificate renewal timer
sudo systemctl status certbot.timer --no-pager

# 5. Start production containers
cd /opt/me-lab
sudo docker compose -f deployments/azure/compose.prod.yaml pull
sudo docker compose -f deployments/azure/compose.prod.yaml up -d

# 6. Verify end-to-end HTTPS readiness
curl -fsS "https://<YOUR_FQDN>/ready"
```

Subsequent tagged releases (`v*.*.*`) deploy automatically through `.github/workflows/deploy-azure.yml` using Azure OIDC federated credentials (`AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`) and `az vm run-command invoke`.

---

## 6. Online SQLite Backup Schedule & Retention

### How `scripts/backup-sqlite.sh` Works
1. Invokes SQLite's online C Backup API (`sqlite3 "$DATABASE_URL" ".backup '$TMP_BACKUP'"`), safely snapshotting `/mnt/melab-data/me-lab.db` **plus any uncheckpointed `-wal` frames** without blocking or corrupting concurrent readers/writers.
2. Runs `PRAGMA integrity_check` on the snapshot before compression.
3. Compresses with `gzip -n -9` (`me-lab-backup-YYYYMMDDTHHMMSSZ.db.gz`) and generates a companion SHA-256 checksum file (`.db.gz.sha256`).
4. Authenticates via the VM's System-Assigned Managed Identity (`az login --identity`) and uploads both artifacts using `az storage blob upload --auth-mode login`.
5. Prunes blobs older than `RETENTION_DAYS` (default **30 days**).

### Schedule
`deployments/azure/cloud-init.yaml` installs `melab-backup.service` and `melab-backup.timer` running daily at **`02:00 UTC`** (`Persistent=true`). Check timer health or trigger an on-demand backup anytime:

```bash
sudo systemctl list-timers melab-backup.timer
sudo systemctl start melab-backup.service
sudo journalctl -u melab-backup.service -n 50 --no-pager
```

---

## 7. Disaster Recovery (DR) Restore Runbook

Follow these exact steps to recover `/mnt/melab-data/me-lab.db` from Azure Blob Storage after accidental data corruption, operator error, or full VM replacement:

```bash
cd /opt/me-lab
set -a && sudo cat /opt/me-lab/.env > /tmp/melab.env && source /tmp/melab.env && rm -f /tmp/melab.env && set +a

# Step 1: List available backups in Azure Blob Storage
az login --identity --output none
az storage blob list \
  --account-name "$AZURE_STORAGE_ACCOUNT" \
  --container-name "$AZURE_STORAGE_CONTAINER" \
  --auth-mode login \
  --query "[?ends_with(name, '.db.gz')].{Name:name, Size:properties.contentLength, Modified:properties.lastModified}" \
  --output table

# Step 2: Stop the API container so no open file descriptors write to me-lab.db-wal
sudo docker compose -f deployments/azure/compose.prod.yaml stop api

# Step 3: Run restore-sqlite.sh (pass specific blob name, or omit to restore the latest snapshot)
# The script downloads .db.gz + .sha256, verifies SHA-256, decompresses, checks
# PRAGMA integrity_check & PRAGMA foreign_key_check, removes stale -wal/-shm files,
# and atomically renames the verified DB into /mnt/melab-data/me-lab.db.
sudo env \
  DATABASE_URL="/mnt/melab-data/me-lab.db" \
  AZURE_STORAGE_ACCOUNT="$AZURE_STORAGE_ACCOUNT" \
  AZURE_STORAGE_CONTAINER="$AZURE_STORAGE_CONTAINER" \
  AZURE_USE_MANAGED_IDENTITY="true" \
  /opt/me-lab/scripts/restore-sqlite.sh "me-lab-backup-20261102T020000Z.db.gz"

# Step 4: Restore melab container UID/GID ownership and start API
sudo chown -R 10001:10001 /mnt/melab-data
sudo docker compose -f deployments/azure/compose.prod.yaml start api

# Step 5: Verify readiness and application health
curl -fsS "https://<YOUR_FQDN>/ready"
curl -fsS "https://<YOUR_FQDN>/health"
```

### Local & CI Backup/Restore Drill
Run the automated end-to-end backup, wipe, restore, and row-equality verification script locally at any time without Azure credentials:

```bash
./scripts/test-backup-restore.sh
```

---

## 8. Azure for Students Cost-Control & VM Deallocation

On an **Azure for Students** ($100 credit) subscription, compute uptime is the primary cost driver:

| Resource | SKU | Approx. Monthly Cost (24/7) | Cost While VM Deallocated |
| --- | --- | --- | --- |
| Linux VM | `Standard_B2s` | ~$30–$35 / mo | **$0.00 / hr** |
| Managed Data Disk | `32 GiB StandardSSD_LRS` | ~$2.40 / mo | ~$2.40 / mo (all SQLite data preserved) |
| Static Public IP + Blob Backups | `Standard` IP + `Standard_LRS` (<1 GiB) | ~$3.65 / mo | ~$3.65 / mo (DNS + backups preserved) |

> **Critical**: Running `sudo shutdown -h now` inside the Linux guest stops the OS **without releasing Azure hardware**, so compute billing continues! Always use **`az vm deallocate`** from the Azure CLI or Portal.

### Deallocate When Idle & Restart Before Demos

```bash
# Stop compute billing completely while preserving /mnt/melab-data and DNS/IP:
az vm deallocate --resource-group "$AZURE_RG" --name "vm-melab-prod"

# Confirm state shows "VM deallocated":
az vm get-instance-view \
  --resource-group "$AZURE_RG" \
  --name "vm-melab-prod" \
  --query "instanceView.statuses[?starts_with(code, 'PowerState/')].displayStatus" \
  --output tsv

# Boot VM back up (~45 seconds; systemd + Docker Compose restart automatically):
az vm start --resource-group "$AZURE_RG" --name "vm-melab-prod"
```

### Optional Daily Auto-Shutdown Safety Net

Configure Azure to automatically deallocate the VM every night at `03:00 UTC` so an unattended demo instance never drains student credits:

```bash
az vm auto-shutdown \
  --resource-group "$AZURE_RG" \
  --name "vm-melab-prod" \
  --time 0300
```
