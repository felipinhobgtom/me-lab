#!/usr/bin/env bash
# Verified SQLite restore script for ME Lab.
# Downloads a compressed backup + SHA-256 checksum from Azure Blob Storage (or
# LOCAL_BACKUP_DIR), verifies the SHA-256 digest, decompresses, validates
# `PRAGMA integrity_check` and `PRAGMA foreign_key_check`, clears stale WAL/SHM
# sidecars, and atomically swaps the verified database into DATABASE_URL.

set -euo pipefail

BACKUP_NAME=""
while [ $# -gt 0 ]; do
  case "$1" in
    --backup|--blob|-b)
      if [ $# -lt 2 ]; then
        echo "ERROR: Missing value for $1" >&2
        exit 1
      fi
      BACKUP_NAME="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [--blob <me-lab-backup-YYYYMMDDTHHMMSSZ.db.gz>]"
      exit 0
      ;;
    *)
      BACKUP_NAME="$1"
      shift
      ;;
  esac
done

DATABASE_URL="${DATABASE_URL:-}"
if [ -z "$DATABASE_URL" ]; then
  echo "ERROR: DATABASE_URL must be set (target SQLite database path)" >&2
  exit 1
fi

TARGET_DIR="$(dirname "$DATABASE_URL")"
mkdir -p "$TARGET_DIR"

TMP_DIR="$(mktemp -d)"
STAGED_DB="${TARGET_DIR}/.me-lab-restore-$$.sqlite.tmp"
cleanup() {
  rm -rf "$TMP_DIR" "$STAGED_DB"
}
trap cleanup EXIT

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

if [ -n "${LOCAL_BACKUP_DIR:-}" ]; then
  if [ ! -d "$LOCAL_BACKUP_DIR" ]; then
    echo "ERROR: LOCAL_BACKUP_DIR=${LOCAL_BACKUP_DIR} does not exist" >&2
    exit 1
  fi
  if [ -z "$BACKUP_NAME" ]; then
    shopt -s nullglob
    archives=("$LOCAL_BACKUP_DIR"/me-lab-backup-*.db.gz)
    shopt -u nullglob
    if [ "${#archives[@]}" -eq 0 ]; then
      echo "ERROR: No backups matching me-lab-backup-*.db.gz found in ${LOCAL_BACKUP_DIR}" >&2
      exit 1
    fi
    BACKUP_NAME="$(basename "${archives[$((${#archives[@]} - 1))]}")"
  fi

  SRC_ARCHIVE="${LOCAL_BACKUP_DIR}/${BACKUP_NAME}"
  SRC_CHECKSUM="${SRC_ARCHIVE}.sha256"
  if [ ! -f "$SRC_ARCHIVE" ] || [ ! -f "$SRC_CHECKSUM" ]; then
    echo "ERROR: Backup or checksum file missing: ${SRC_ARCHIVE} (.sha256)" >&2
    exit 1
  fi

  TMP_ARCHIVE="${TMP_DIR}/${BACKUP_NAME}"
  TMP_CHECKSUM="${TMP_DIR}/${BACKUP_NAME}.sha256"
  cp -f "$SRC_ARCHIVE" "$TMP_ARCHIVE"
  cp -f "$SRC_CHECKSUM" "$TMP_CHECKSUM"
else
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
    if [ -z "$BACKUP_NAME" ]; then
      BACKUP_NAME="$(az storage blob list \
        --account-name "$AZURE_STORAGE_ACCOUNT" \
        --container-name "$AZURE_STORAGE_CONTAINER" \
        --prefix "me-lab-backup-" \
        --auth-mode login \
        --query "sort_by([?ends_with(name, '.db.gz')], &name)[-1].name" \
        --output tsv)"
    fi
    if [ -z "$BACKUP_NAME" ] || [ "$BACKUP_NAME" = "None" ]; then
      echo "ERROR: No backup blobs found in ${AZURE_STORAGE_ACCOUNT}/${AZURE_STORAGE_CONTAINER}" >&2
      exit 1
    fi

    TMP_ARCHIVE="${TMP_DIR}/${BACKUP_NAME}"
    TMP_CHECKSUM="${TMP_DIR}/${BACKUP_NAME}.sha256"

    az storage blob download \
      --account-name "$AZURE_STORAGE_ACCOUNT" \
      --container-name "$AZURE_STORAGE_CONTAINER" \
      --name "$BACKUP_NAME" \
      --file "$TMP_ARCHIVE" \
      --auth-mode login \
      --output none

    az storage blob download \
      --account-name "$AZURE_STORAGE_ACCOUNT" \
      --container-name "$AZURE_STORAGE_CONTAINER" \
      --name "${BACKUP_NAME}.sha256" \
      --file "$TMP_CHECKSUM" \
      --auth-mode login \
      --output none
  elif [ -n "${AZURE_STORAGE_SAS_TOKEN:-}" ]; then
    if [ -z "$BACKUP_NAME" ]; then
      BACKUP_NAME="$(az storage blob list \
        --account-name "$AZURE_STORAGE_ACCOUNT" \
        --container-name "$AZURE_STORAGE_CONTAINER" \
        --prefix "me-lab-backup-" \
        --sas-token "$AZURE_STORAGE_SAS_TOKEN" \
        --query "sort_by([?ends_with(name, '.db.gz')], &name)[-1].name" \
        --output tsv)"
    fi
    if [ -z "$BACKUP_NAME" ] || [ "$BACKUP_NAME" = "None" ]; then
      echo "ERROR: No backup blobs found in ${AZURE_STORAGE_ACCOUNT}/${AZURE_STORAGE_CONTAINER}" >&2
      exit 1
    fi

    TMP_ARCHIVE="${TMP_DIR}/${BACKUP_NAME}"
    TMP_CHECKSUM="${TMP_DIR}/${BACKUP_NAME}.sha256"

    az storage blob download \
      --account-name "$AZURE_STORAGE_ACCOUNT" \
      --container-name "$AZURE_STORAGE_CONTAINER" \
      --name "$BACKUP_NAME" \
      --file "$TMP_ARCHIVE" \
      --sas-token "$AZURE_STORAGE_SAS_TOKEN" \
      --output none

    az storage blob download \
      --account-name "$AZURE_STORAGE_ACCOUNT" \
      --container-name "$AZURE_STORAGE_CONTAINER" \
      --name "${BACKUP_NAME}.sha256" \
      --file "$TMP_CHECKSUM" \
      --sas-token "$AZURE_STORAGE_SAS_TOKEN" \
      --output none
  else
    echo "ERROR: AZURE_USE_MANAGED_IDENTITY is false and AZURE_STORAGE_SAS_TOKEN is empty" >&2
    exit 1
  fi
fi

echo "[restore] Verifying SHA-256 checksum for ${BACKUP_NAME}..."
EXPECTED_SHA256="$(awk '{print $1}' "$TMP_CHECKSUM")"
ACTUAL_SHA256="$(compute_sha256 "$TMP_ARCHIVE")"
if [ -z "$EXPECTED_SHA256" ] || [ "$EXPECTED_SHA256" != "$ACTUAL_SHA256" ]; then
  echo "ERROR: SHA-256 mismatch for ${BACKUP_NAME} (expected ${EXPECTED_SHA256}, got ${ACTUAL_SHA256})" >&2
  exit 1
fi

echo "[restore] Decompressing and running SQLite integrity & foreign key checks..."
gzip -dc "$TMP_ARCHIVE" > "$STAGED_DB"

INTEGRITY="$(sqlite3 "$STAGED_DB" "PRAGMA integrity_check;")"
if [ "$INTEGRITY" != "ok" ]; then
  echo "ERROR: Restored database failed PRAGMA integrity_check: ${INTEGRITY}" >&2
  exit 1
fi

FK_VIOLATIONS="$(sqlite3 "$STAGED_DB" "PRAGMA foreign_key_check;")"
if [ -n "$FK_VIOLATIONS" ]; then
  echo "ERROR: Restored database failed PRAGMA foreign_key_check: ${FK_VIOLATIONS}" >&2
  exit 1
fi

if [ "${KEEP_PRE_RESTORE_COPY:-false}" = "true" ] && [ -f "$DATABASE_URL" ]; then
  PRE_COPY="${DATABASE_URL}.pre-restore-$(date -u +"%Y%m%dT%H%M%SZ").bak"
  cp -f "$DATABASE_URL" "$PRE_COPY"
  echo "[restore] Saved pre-restore safety copy at ${PRE_COPY}"
fi

# Remove stale WAL/SHM sidecar files before atomic POSIX rename on target filesystem
rm -f "${DATABASE_URL}-wal" "${DATABASE_URL}-shm"
mv -f "$STAGED_DB" "$DATABASE_URL"

echo "[restore] Successfully restored ${BACKUP_NAME} -> ${DATABASE_URL}"
