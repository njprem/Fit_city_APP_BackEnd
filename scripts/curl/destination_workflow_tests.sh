#!/usr/bin/env bash

set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
AUTHOR_TOKEN=${AUTHOR_TOKEN:-}
REVIEWER_TOKEN=${REVIEWER_TOKEN:-}
HISTORY_FILE=${HISTORY_FILE:-scripts/curl/last_destination_commands.log}
UPLOAD_SOURCE_DIR=${UPLOAD_SOURCE_DIR:-$HOME/Pictures/BingWallpaper}
DEST_PUBLIC_BASE=${DEST_PUBLIC_BASE:-http://localhost:9000/fitcity-destinations}
REAL_UPLOAD_ENABLED=${REAL_UPLOAD_ENABLED:-1}
UPLOAD_MAX_BYTES=${UPLOAD_MAX_BYTES:-5242880}
MC_BINARY=${MC_BINARY:-scripts/bin/mc}
MC_URL=${MC_URL:-https://dl.min.io/client/mc/release/linux-amd64/mc}
CREATE_SUCCESS_PHOTO_COUNT=${CREATE_SUCCESS_PHOTO_COUNT:-3}
CREATE_REJECT_PHOTO_COUNT=${CREATE_REJECT_PHOTO_COUNT:-1}

if [[ -z "$AUTHOR_TOKEN" || -z "$REVIEWER_TOKEN" ]]; then
  echo "Set AUTHOR_TOKEN and REVIEWER_TOKEN environment variables before running this script." >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for this script." >&2
  exit 1
fi

mkdir -p "$(dirname "$HISTORY_FILE")"

CURRENT_SCENARIO=""
MC=""
DEST_PUBLIC_BASE=${DEST_PUBLIC_BASE%/}

start_scenario_log() {
  CURRENT_SCENARIO="$1"
  {
    echo ""
    echo "### $(date -Iseconds) $CURRENT_SCENARIO"
  } >>"$HISTORY_FILE"
}

log_command() {
  local -a cmd=("$@")
  local line
  printf -v line '%q ' "${cmd[@]}"
  line=${line% }
  echo "$line" >>"$HISTORY_FILE"
}

request() {
  local method=$1
  local path=$2
  local token=${3:-}
  local data=${4:-}

  local -a cmd=(curl -sS -X "$method" "$BASE_URL$path" -H "Accept: application/json" -w '\n')
  if [[ -n "$token" ]]; then
    cmd+=(-H "Authorization: Bearer $token")
  fi
  if [[ -n "$data" ]]; then
    cmd+=(-H "Content-Type: application/json" -d "$data")
  fi

  printf -v pretty '%q ' "${cmd[@]}"
  pretty=${pretty% }
  echo "[curl] $pretty" >&2
  log_command "${cmd[@]}"
  "${cmd[@]}"
}

create_change() {
  local payload=$1
  request POST "/api/v1/admin/destination-changes" "$AUTHOR_TOKEN" "$payload"
}

submit_change() {
  local change_id=$1
  request POST "/api/v1/admin/destination-changes/$change_id/submit" "$AUTHOR_TOKEN" ""
}

approve_change() {
  local change_id=$1
  request POST "/api/v1/admin/destination-changes/$change_id/approve" "$REVIEWER_TOKEN" ""
}

reject_change() {
  local change_id=$1
  local message=$2
  local body
  body=$(jq -n --arg msg "$message" '{message: $msg}' | jq -c '.')
  request POST "/api/v1/admin/destination-changes/$change_id/reject" "$REVIEWER_TOKEN" "$body"
}

random_suffix() {
  printf '%s%s' "$(date +%s%N)" "$RANDOM"
}

slugify() {
  local input=$1
  input=$(echo "$input" | tr '[:upper:]' '[:lower:]')
  input=$(echo "$input" | sed -E 's/[^a-z0-9]+/-/g; s/^-+|-+$//g')
  echo "$input"
}

gallery_payload() {
  local prefix=$1
  jq -n --arg prefix "$prefix" '[
    {url: ($prefix + "-1.jpg"), ordering: 1},
    {url: ($prefix + "-2.jpg"), ordering: 2, caption: "Evening skyline"}
  ]'
}

build_create_payload() {
  local name=$1
  local status=${2:-published}
  local suffix=${3:-$(random_suffix)}
  local slug
  slug="$(slugify "$name")-$suffix"
  local gallery_json
  if [[ "$REAL_UPLOAD_ENABLED" == "1" ]]; then
    gallery_json="null"
  else
    gallery_json=$(gallery_payload "https://cdn.fitcity.local/$slug")
  fi
  jq -n \
    --arg name "$name" \
    --arg slug "$slug" \
    --arg category "Nature" \
    --arg description "Auto-generated destination $name ($suffix)" \
    --arg contact "+1-555-${suffix: -4}" \
    --arg opening "08:00" \
    --arg closing "21:30" \
    --arg status "$status" \
    --argjson gallery "$gallery_json" '
      {
        action: "create",
        fields: {
          name: $name,
          slug: $slug,
          category: $category,
          description: $description,
          contact: $contact,
          opening_time: $opening,
          closing_time: $closing
        }
      }
      | if $gallery == null then . else .fields.gallery = $gallery end
      | if $status == "" then . else .fields.status = $status end
    '
}

build_update_payload() {
  local destination_id=$1
  local new_status=${2:-}
  local suffix=${3:-$(random_suffix)}
  local gallery_json
  if [[ "$REAL_UPLOAD_ENABLED" == "1" ]]; then
    gallery_json="null"
  else
    gallery_json=$(gallery_payload "https://cdn.fitcity.local/$destination_id/$suffix")
  fi
  jq -n \
    --arg destinationId "$destination_id" \
    --arg description "Updated description $suffix" \
    --arg contact "+1-555-9${suffix: -4}" \
    --arg opening "09:00" \
    --arg closing "23:00" \
    --arg status "$new_status" \
    --argjson gallery "$gallery_json" '
      {
        action: "update",
        destination_id: $destinationId,
        fields: {
          description: $description,
          contact: $contact,
          opening_time: $opening,
          closing_time: $closing
        }
      }
      | if $gallery == null then . else .fields.gallery = $gallery end
      | if $status == "" then . else .fields.status = $status end
    '
}

build_delete_payload() {
  local destination_id=$1
  local hard_delete=${2:-false}
  jq -n \
    --arg destinationId "$destination_id" \
    --argjson hard "$hard_delete" '
      {
        action: "delete",
        destination_id: $destinationId,
        fields: {
          hard_delete: $hard
        }
      }
    '
}

ensure_upload_source() {
  if [[ "$REAL_UPLOAD_ENABLED" != "1" ]]; then
    return
  fi
  if [[ ! -d "$UPLOAD_SOURCE_DIR" ]]; then
    echo "Upload source directory not found: $UPLOAD_SOURCE_DIR" >&2
    exit 1
  fi
  if ! find "$UPLOAD_SOURCE_DIR" -type f -print -quit >/dev/null; then
    echo "No uploadable files found in $UPLOAD_SOURCE_DIR" >&2
    exit 1
  fi
}

ensure_mc() {
  if [[ -n "$MC" && -x "$MC" ]]; then
    return
  fi
  if command -v mc >/dev/null 2>&1; then
    MC=$(command -v mc)
    return
  fi
  MC=$MC_BINARY
  if [[ ! -x "$MC" ]]; then
    mkdir -p "$(dirname "$MC")"
    echo "Downloading MinIO client to $MC" >&2
    curl -sSL "$MC_URL" -o "$MC"
    chmod +x "$MC"
  fi
}

ensure_minio_alias() {
  ensure_mc
  log_command "$MC" alias set fitcity http://localhost:9000 minioadmin hKzeWB2Am1scFVSP
  "$MC" alias set fitcity http://localhost:9000 minioadmin hKzeWB2Am1scFVSP >/dev/null
}

ensure_minio_bucket() {
  ensure_minio_alias
  if ! "$MC" ls fitcity/fitcity-destinations >/dev/null 2>&1; then
    log_command "$MC" mb fitcity/fitcity-destinations
    "$MC" mb fitcity/fitcity-destinations >/dev/null
  fi
}

get_random_files() {
  local count=$1
  ensure_upload_source
  local output
if ! output=$(COUNT="$count" UPLOAD_SOURCE_DIR="$UPLOAD_SOURCE_DIR" UPLOAD_MAX_BYTES="$UPLOAD_MAX_BYTES" python - <<'PY'
import os, random, sys

count = int(os.environ["COUNT"])
root = os.environ["UPLOAD_SOURCE_DIR"]
max_bytes = int(os.environ.get("UPLOAD_MAX_BYTES", "5242880"))
allowed = {".jpg", ".jpeg", ".png", ".webp"}
files = []
for dirpath, dirnames, filenames in os.walk(root):
    dirnames[:] = [d for d in dirnames if not d.startswith('.')]
    for name in filenames:
        if name.startswith('.'):
            continue
        ext = os.path.splitext(name)[1].lower()
        if allowed and ext not in allowed:
            continue
        path = os.path.join(dirpath, name)
        try:
            size = os.path.getsize(path)
        except OSError:
            continue
        if size == 0 or size > max_bytes:
            continue
        files.append(path)
if len(files) < count:
    sys.stderr.write(
        f"Requested {count} files but only found {len(files)} usable files (<= {max_bytes} bytes) in {root}\n"
    )
    sys.exit(1)
random.shuffle(files)
for path in files[:count]:
    print(path)
PY
); then
    exit 1
  fi
  printf '%s\n' "$output"
}

upload_hero_image_file() {
  local change_id=$1
  local file_path=$2
  local filename mime
  filename=$(basename "$file_path")
  if command -v file >/dev/null 2>&1; then
    mime=$(file --brief --mime-type "$file_path")
  else
    mime="image/jpeg"
  fi
  local -a cmd=(
    curl -sS -X POST "$BASE_URL/api/v1/admin/destination-changes/$change_id/hero-image"
    -H "Accept: application/json"
    -H "Authorization: Bearer $AUTHOR_TOKEN"
    -F "file=@${file_path};filename=${filename};type=${mime}"
    -w '\n'
  )
  printf -v pretty '%q ' "${cmd[@]}"
  pretty=${pretty% }
  echo "[curl] $pretty" >&2
  log_command "${cmd[@]}"
  "${cmd[@]}"
}

update_change_fields() {
  local change_id=$1
  local draft_version=$2
  local fields_json=$3
  local body
  body=$(jq -c -n --argjson dv "$draft_version" --argjson fields "$fields_json" '{draft_version: $dv, fields: $fields}')
  request PUT "/api/v1/admin/destination-changes/$change_id" "$AUTHOR_TOKEN" "$body"
}

upload_gallery_files() {
  local change_id=$1
  shift
  if [[ $# -eq 0 ]]; then
    echo '[]'
    return
  fi
  ensure_minio_bucket
  local gallery='[]'
  local ordering=1
  for file_path in "$@"; do
    local basename ext uuid object_key
    basename=$(basename "$file_path")
    ext=${basename##*.}
    ext=${ext,,}
    if [[ -z "$ext" || "$ext" == "$basename" ]]; then
      ext="jpg"
    fi
    uuid=$(uuidgen | tr 'A-Z' 'a-z')
    object_key="destinations/changes/$change_id/gallery/${uuid}.${ext}"
    echo "[gallery] Uploading image #$ordering -> $object_key" >&2
    log_command "$MC" cp "$file_path" "fitcity/fitcity-destinations/$object_key"
    "$MC" cp "$file_path" "fitcity/fitcity-destinations/$object_key" >/dev/null
    local url="$DEST_PUBLIC_BASE/$object_key"
    gallery=$(echo "$gallery" | jq --arg url "$url" --arg caption "$basename" --argjson ordering "$ordering" '. + [{url: $url, caption: $caption, ordering: $ordering}]')
    ordering=$((ordering + 1))
  done
  echo "$gallery"
}

apply_real_uploads() {
  local change_id=$1
  local draft_version=$2
  local photo_count=${3:-1}
  local label=${4:-uploads}
  if [[ "$REAL_UPLOAD_ENABLED" != "1" ]]; then
    echo "$draft_version"
    return
  fi
  mapfile -t photo_files < <(get_random_files "$photo_count")
  if [[ "${#photo_files[@]}" -eq 0 ]]; then
    echo "$draft_version"
    return
  fi

  echo "[$label] Uploading hero image: ${photo_files[0]}" >&2
  local hero_resp
  hero_resp=$(upload_hero_image_file "$change_id" "${photo_files[0]}")
  local hero_change_id
  hero_change_id=$(echo "$hero_resp" | jq -r '.change_request.id // empty')
  if [[ "$hero_change_id" != "$change_id" ]]; then
    echo "[$label] Hero upload failed: $hero_resp" >&2
    exit 1
  fi
  draft_version=$(echo "$hero_resp" | jq -r '.change_request.draft_version')
  if [[ -z "$draft_version" || "$draft_version" == "null" ]]; then
    echo "[$label] Unable to determine draft version after hero upload: $hero_resp" >&2
    exit 1
  fi
  local hero_url
  hero_url=$(echo "$hero_resp" | jq -r '.change_request.fields.hero_image_url // empty')
  if [[ -z "$hero_url" ]]; then
    echo "[$label] Hero upload did not return a public URL" >&2
    exit 1
  fi
  echo "[$label] Hero image URL: $hero_url" >&2

  if (( photo_count > 1 )); then
    local gallery_files=("${photo_files[@]:1}")
    local gallery_json
    gallery_json=$(upload_gallery_files "$change_id" "${gallery_files[@]}")
    if [[ "$gallery_json" != "[]" ]]; then
      local base_name
      base_name=$(echo "$hero_resp" | jq -r '.change_request.fields.name // empty')
      if [[ -z "$base_name" ]]; then
        echo "[$label] Unable to determine destination name for gallery update" >&2
        exit 1
      fi
      local fields_update
      fields_update=$(jq -n --arg name "$base_name" --argjson gallery "$gallery_json" '{name: $name, gallery: $gallery}')
      local update_resp
      update_resp=$(update_change_fields "$change_id" "$draft_version" "$fields_update")
      local update_change_id
      update_change_id=$(echo "$update_resp" | jq -r '.change_request.id // empty')
      if [[ "$update_change_id" != "$change_id" ]]; then
        echo "[$label] Gallery update failed: $update_resp" >&2
        exit 1
      fi
      draft_version=$(echo "$update_resp" | jq -r '.change_request.draft_version')
      if [[ -z "$draft_version" || "$draft_version" == "null" ]]; then
        echo "[$label] Unable to determine draft version after gallery update: $update_resp" >&2
        exit 1
      fi
      echo "[$label] Attached ${#gallery_files[@]} gallery images" >&2
    fi
  fi

  echo "$draft_version"
}

create_destination_record() {
  local name=$1
  local status=$2
  local payload_json
  payload_json=$(build_create_payload "$name" "$status" | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  local draft_version
  draft_version=$(echo "$create_resp" | jq -r '.change_request.draft_version')
  draft_version=$(apply_real_uploads "$change_id" "$draft_version" 2 "seed:$name")
  submit_change "$change_id" "" >/dev/null
  approve_change "$change_id"
}

scenario_create_publish_success() {
  start_scenario_log "create_publish_success"
  echo "== Create -> Submit -> Approve (published) =="
  local payload_json
  payload_json=$(build_create_payload "Aurora Park" "published" | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Draft change: $change_id"
  local draft_version
  draft_version=$(echo "$create_resp" | jq -r '.change_request.draft_version')
  draft_version=$(apply_real_uploads "$change_id" "$draft_version" "$CREATE_SUCCESS_PHOTO_COUNT" "create_publish_success")

  submit_change "$change_id" "" >/dev/null
  echo "Submitted for review."

  local approve_resp
  approve_resp=$(approve_change "$change_id")
  local destination_id
  destination_id=$(echo "$approve_resp" | jq -r '.destination.id')
  echo "Published destination: $destination_id"
  echo "$approve_resp" | jq '.destination'

  request GET "/api/v1/destinations/$destination_id" "" "" | jq '.'
}

scenario_create_publish_reject() {
  start_scenario_log "create_publish_reject"
  echo "== Create -> Submit -> Reject =="
  local payload_json
  payload_json=$(build_create_payload "Rejected Harbor" "draft" | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Draft change: $change_id"
  local draft_version
  draft_version=$(echo "$create_resp" | jq -r '.change_request.draft_version')
  draft_version=$(apply_real_uploads "$change_id" "$draft_version" "$CREATE_REJECT_PHOTO_COUNT" "create_publish_reject")

  submit_change "$change_id" "" >/dev/null
  echo "Submitted for review."

  local reject_resp
  reject_resp=$(reject_change "$change_id" "Additional documentation required.")
  echo "Rejected change status:"
  echo "$reject_resp" | jq '.change_request'
}

scenario_update_published_success() {
  start_scenario_log "update_published_success"
  echo "== Update Published -> Approve =="
  local seed_resp
  seed_resp=$(create_destination_record "Published Base $(random_suffix)" "published")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Seed destination: $destination_id"

  local payload_json
  payload_json=$(build_update_payload "$destination_id" "" | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Update change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted update."

  local approve_resp
  approve_resp=$(approve_change "$change_id")
  echo "Approved update:"
  echo "$approve_resp" | jq '.destination'
}

scenario_update_published_reject() {
  start_scenario_log "update_published_reject"
  echo "== Update Published -> Reject =="
  local seed_resp
  seed_resp=$(create_destination_record "Published Reject $(random_suffix)" "published")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Seed destination: $destination_id"

  local payload_json
  payload_json=$(build_update_payload "$destination_id" "" | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Update change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted update."

  local reject_resp
  reject_resp=$(reject_change "$change_id" "Revert contact details.")
  echo "Rejected update status:"
  echo "$reject_resp" | jq '.change_request'
}

scenario_update_draft_success() {
  start_scenario_log "update_draft_success"
  echo "== Update Draft -> Approve (publish) =="
  local seed_resp
  seed_resp=$(create_destination_record "Draft Base $(random_suffix)" "draft")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Seed draft destination: $destination_id"

  local payload_json
  payload_json=$(build_update_payload "$destination_id" "published" | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Update change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted update."

  local approve_resp
  approve_resp=$(approve_change "$change_id")
  echo "Approved update promoted to published:"
  echo "$approve_resp" | jq '.destination'
}

scenario_update_draft_reject() {
  start_scenario_log "update_draft_reject"
  echo "== Update Draft -> Reject =="
  local seed_resp
  seed_resp=$(create_destination_record "Draft Reject $(random_suffix)" "draft")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Seed draft destination: $destination_id"

  local payload_json
  payload_json=$(build_update_payload "$destination_id" "" | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Update change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted update."

  local reject_resp
  reject_resp=$(reject_change "$change_id" "Hold draft for further review.")
  echo "Rejected update:"
  echo "$reject_resp" | jq '.change_request'
}

scenario_delete_published_success() {
  start_scenario_log "delete_published_success"
  echo "== Delete Published -> Approve (archive) =="
  local seed_resp
  seed_resp=$(create_destination_record "Published Delete $(random_suffix)" "published")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Published destination: $destination_id"

  local payload_json
  payload_json=$(build_delete_payload "$destination_id" false | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Delete change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted deletion."

  local approve_resp
  approve_resp=$(approve_change "$change_id")
  echo "Archive result:"
  echo "$approve_resp" | jq '.destination'
}

scenario_delete_published_reject() {
  start_scenario_log "delete_published_reject"
  echo "== Delete Published -> Reject =="
  local seed_resp
  seed_resp=$(create_destination_record "Published Delete Reject $(random_suffix)" "published")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Published destination: $destination_id"

  local payload_json
  payload_json=$(build_delete_payload "$destination_id" false | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Delete change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted deletion."

  local reject_resp
  reject_resp=$(reject_change "$change_id" "Keep destination live for now.")
  echo "Rejected deletion:"
  echo "$reject_resp" | jq '.change_request'
}

scenario_delete_draft_success() {
  start_scenario_log "delete_draft_success"
  echo "== Delete Draft -> Approve (archive) =="
  local seed_resp
  seed_resp=$(create_destination_record "Draft Delete $(random_suffix)" "draft")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Draft destination: $destination_id"

  local payload_json
  payload_json=$(build_delete_payload "$destination_id" false | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Delete change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted deletion."

  local approve_resp
  approve_resp=$(approve_change "$change_id")
  echo "Archive result:"
  echo "$approve_resp" | jq '.destination'
}

scenario_delete_draft_reject() {
  start_scenario_log "delete_draft_reject"
  echo "== Delete Draft -> Reject =="
  local seed_resp
  seed_resp=$(create_destination_record "Draft Delete Reject $(random_suffix)" "draft")
  local destination_id
  destination_id=$(echo "$seed_resp" | jq -r '.destination.id')
  echo "Draft destination: $destination_id"

  local payload_json
  payload_json=$(build_delete_payload "$destination_id" false | jq -c '.')
  local create_resp
  create_resp=$(create_change "$payload_json")
  local change_id
  change_id=$(echo "$create_resp" | jq -r '.change_request.id')
  echo "Delete change: $change_id"

  submit_change "$change_id" "" >/dev/null
  echo "Submitted deletion."

  local reject_resp
  reject_resp=$(reject_change "$change_id" "Retain draft for future iteration.")
  echo "Rejected deletion:"
  echo "$reject_resp" | jq '.change_request'
}

run_all() {
  scenario_create_publish_success
  scenario_create_publish_reject
  scenario_update_published_success
  scenario_update_published_reject
  scenario_update_draft_success
  scenario_update_draft_reject
  scenario_delete_published_success
  scenario_delete_published_reject
  scenario_delete_draft_success
  scenario_delete_draft_reject
}

list_scenarios() {
  cat <<'EOF'
create_publish_success
create_publish_reject
update_published_success
update_published_reject
update_draft_success
update_draft_reject
delete_published_success
delete_published_reject
delete_draft_success
delete_draft_reject
run_all
EOF
}

usage() {
  cat <<'EOF'
Usage:
  destination_workflow_tests.sh run_all        # execute every scenario (default)
  destination_workflow_tests.sh <scenario>     # run a single scenario (see list)
  destination_workflow_tests.sh list           # show available scenario names

Environment:
  BASE_URL        (default http://localhost:8080)
  AUTHOR_TOKEN    Bearer token for authoring admin (required)
  REVIEWER_TOKEN  Bearer token for reviewer admin (required)
  UPLOAD_SOURCE_DIR  Directory containing images (default ~/Pictures/BingWallpaper)
  DEST_PUBLIC_BASE   Public base URL for destination media (default http://localhost:9000/fitcity-destinations)
  REAL_UPLOAD_ENABLED  Set to 0 to skip real uploads
  UPLOAD_MAX_BYTES   Max file size (bytes) when picking random images (default 5242880)
  CREATE_SUCCESS_PHOTO_COUNT  Total photos (hero + gallery) for create publish success (default 3)
  CREATE_REJECT_PHOTO_COUNT   Total photos for create reject scenario (default 1)
  HISTORY_FILE     Where to append executed curl commands (default scripts/curl/last_destination_commands.log)
EOF
}

main() {
  local cmd=${1:-run_all}
  if [[ "$REAL_UPLOAD_ENABLED" == "1" ]]; then
    ensure_upload_source
  fi
  case "$cmd" in
    run_all) run_all ;;
    list) list_scenarios ;;
    create_publish_success) scenario_create_publish_success ;;
    create_publish_reject) scenario_create_publish_reject ;;
    update_published_success) scenario_update_published_success ;;
    update_published_reject) scenario_update_published_reject ;;
    update_draft_success) scenario_update_draft_success ;;
    update_draft_reject) scenario_update_draft_reject ;;
    delete_published_success) scenario_delete_published_success ;;
    delete_published_reject) scenario_delete_published_reject ;;
    delete_draft_success) scenario_delete_draft_success ;;
    delete_draft_reject) scenario_delete_draft_reject ;;
    *) usage >&2; exit 1 ;;
  esac
}

main "$@"
