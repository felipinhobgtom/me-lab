#!/usr/bin/env bash
# Online WAL-safe SQLite backup script for ME Lab.
# Performs an atomic `sqlite3 .backup`, verifies snapshot integrity, compresses
# with gzip, generates a SHA-256 checksum, uploads to Azure Blob Storage via
# Managed Identity (`--auth-mode login`) or copies to LOCAL_BACKUP_DIR, and
# prunes expired backups.

set -euo pipefail

DATABASE_URL="${DATABASE_URL:-}"
if [ -z "$DATABASE_URL" ]; then
  echo "ERROR: DATABASE_URL must be set (e.g., /mnt/melab-data/me-lab.db)" >&2
  exit 1
fi

if [ ! -f "$DATABASE_URL" ]; then
  echo "ERROR: SQLite database file does not exist at DATABASE_URL=${DATABASE_URL}" >&2
  exit 1
fi

RETENTION_DAYS="${RETENTION_DAYS:-30}"
RETENTION_COUNT="${RETENTION_COUNT:-0}"
TIMESTAMP="${BACKUP_TIMESTAMP:-$(date -u +"%Y%m%dT%H%M%SZ")}"
ARCHIVE_NAME="${BACKUP_NAME:-me-lab-backup-${TIMESTAMP}.db.gz}"
CHECKSUM_NAME="${ARCHIVE_NAME}.sha256"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

TMP_BACKUP="${TMP_DIR}/me-lab-snapshot.db"
TMP_ARCHIVE="${TMP_DIR}/${ARCHIVE_NAME}"
TMP_CHECKSUM="${TMP_DIR}/${CHECKSUM_NAME}"

compute_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "ERROR: Neither sha256sum nor shasum is installed" >&2
    exit 1
  fi
}

echo "[backup] Starting online WAL-safe SQLite backup from ${DATABASE_URL}..."
sqlite3 "$DATABASE_URL" ".backup '$TMP_BACKUP'"

INTEGRITY="$(sqlite3 "$TMP_BACKUP" "PRAGMA integrity_check;")"
if [ "$INTEGRITY" != "ok" ]; then
  echo "ERROR: Backup snapshot failed PRAGMA integrity_check: ${INTEGRITY}" >&2
  exit 1
fi

gzip -n -9 -c "$TMP_BACKUP" > "$TMP_ARCHIVE"
rm -f "$TMP_BACKUP"

SHA256_HEX="$(compute_sha256 "$TMP_ARCHIVE")"
printf '%s  %s\n' "$SHA256_HEX" "$ARCHIVE_NAME" > "$TMP_CHECKSUM"
echo "[backup] Created ${ARCHIVE_NAME} (sha256=${SHA256_HEX})"

if [ -n "${LOCAL_BACKUP_DIR:-}" ]; then
  mkdir -p "$LOCAL_BACKUP_DIR"
  cp -f "$TMP_ARCHIVE" "${LOCAL_BACKUP_DIR}/${ARCHIVE_NAME}"
  cp -f "$TMP_CHECKSUM" "${LOCAL_BACKUP_DIR}/${CHECKSUM_NAME}"
  echo "[backup] Stored backup artifacts in LOCAL_BACKUP_DIR=${LOCAL_BACKUP_DIR}"

  # Age-based retention pruning
  find "$LOCAL_BACKUP_DIR" -maxdepth 1 -type f -name 'me-lab-backup-*.db.gz*' -mtime +"$RETENTION_DAYS" -delete

  # Optional count-based retention pruning
  if [ "$RETENTION_COUNT" -gt 0 ]; then
    shopt -s nullglob
    archives=("$LOCAL_BACKUP_DIR"/me-lab-backup-*.db.gz)
    shopt -u nullglob
    total="${#archives[@]}"
    if [ "$total" -gt "$RETENTION_COUNT" ]; then
      excess=$((total - RETENTION_COUNT))
      for ((i = 0; i < excess; i++)); do
        echo "[backup] Pruning old local backup $(basename "${archives[$i]}")"
        rm -f "${archives[$i]}" "${archives[$i]}.sha256"
      done
    fi
  fi
  exit 0
fi

AZURE_STORAGE_ACCOUNT="${AZURE_STORAGE_ACCOUNT:-}"
AZURE_STORAGE_CONTAINER="${AZURE_STORAGE_CONTAINER:-sqlite-backups}"
AZURE_USE_MANAGED_IDENTITY="${AZURE_USE_MANAGED_IDENTITY:-true}"

if [ -z "$AZURE_STORAGE_ACCOUNT" ]; then
  echo "ERROR: Either LOCAL_BACKUP_DIR or AZURE_STORAGE_ACCOUNT must be set" >&2
  exit 1
fi

if [ "$AZURE_USE_MANAGED_IDENTITY" = "true" ]; then
  if ! az account show >/dev/null 2>&1; then
    az login --identity --output none
  fi
  az storage blob upload \
    --account-name "$AZURE_STORAGE_ACCOUNT" \
    --container-name "$AZURE_STORAGE_CONTAINER" \
    --name "$ARCHIVE_NAME" \
    --file "$TMP_ARCHIVE" \
    --auth-mode login \
    --overwrite true \
    --output none

  az storage blob upload \
    --account-name "$AZURE_STORAGE_ACCOUNT" \
    --container-name "$AZURE_STORAGE_CONTAINER" \
    --name "$CHECKSUM_NAME" \
    --file "$TMP_CHECKSUM" \
    --auth-mode login \
    --overwrite true \
    --output none
elif [ -n "${AZURE_STORAGE_SAS_TOKEN:-}" ]; then
  az storage blob upload \
    --account-name "$AZURE_STORAGE_ACCOUNT" \
    --container-name "$AZURE_STORAGE_CONTAINER" \
    --name "$ARCHIVE_NAME" \
    --file "$TMP_ARCHIVE" \
    --sas-token "$AZURE_STORAGE_SAS_TOKEN" \
    --overwrite true \
    --output none

  az storage blob upload \
    --account-name "$AZURE_STORAGE_ACCOUNT" \
    --container-name "$AZURE_STORAGE_CONTAINER" \
    --name "$CHECKSUM_NAME" \
    --file "$TMP_CHECKSUM" \
    --sas-token "$AZURE_STORAGE_SAS_TOKEN" \
    --overwrite true \
    --output none
else
  echo "ERROR: AZURE_USE_MANAGED_IDENTITY is false and AZURE_STORAGE_SAS_TOKEN is empty" >&2
  exit 1
fi

echo "[backup] Uploaded ${ARCHIVE_NAME} and ${CHECKSUM_NAME} to ${AZURE_STORAGE_ACCOUNT}/${AZURE_STORAGE_CONTAINER}"

# Retention pruning on Azure Blob Storage
if CUTOFF_ISO="$(date -u -d "-${RETENTION_DAYS} days" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -v-"${RETENTION_DAYS}"d +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)"; then
  EXPIRED_BLOBS="$(az storage blob list \
    --account-name "$AZURE_STORAGE_ACCOUNT" \
    --container-name "$AZURE_STORAGE_CONTAINER" \
    --prefix "me-lab-backup-" \
    --auth-mode login \
    --query "[?properties.lastModified < '${CUTOFF_ISO}'].name" \
    --output tsv)"
  if [ -n "$EXPIRED_BLOBS" ]; then
    while IFS= read -r blob_name; do
      [ -z "$blob_name" ] && continue
      echo "[backup] Pruning expired blob ${blob_name}"
      az storage blob delete \
        --account-name "$AZURE_STORAGE_ACCOUNT" \
        --container-name "$AZURE_STORAGE_CONTAINER" \
        --name "$blob_name" \
        --auth-mode login \
        --output none
    done <<< "$EXPIRED_BLOBS"
  fi
fi
