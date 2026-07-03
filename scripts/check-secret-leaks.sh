#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT

git ls-files -co --exclude-standard -z \
  | while IFS= read -r -d '' file; do
      case "$file" in
        conf/app.mozia.conf|docker-compose.mozia.yml|.env.mozia.example)
          ;;
        *)
          continue
          ;;
      esac
      [ -f "$file" ] || continue
      if LC_ALL=C grep -Iq . "$file"; then
        printf '%s\0' "$file"
      fi
    done \
  | xargs -0 awk '
      BEGIN {
        credential_url = "(postgres(ql)?|mysql|redis|mongodb|amqp|clickhouse)://[^[:space:]/:@]+:[^[:space:]@]+@"
        pg_kv_password = "password=[^[:space:]]+"
        sensitive_key = "(dataSourceName|redisEndpoint|radiusSecret|SQL_DSN|DATABASE_URL|POSTGRES_PASSWORD|PGPASSWORD|DB_PASSWORD|ACCESS_TOKEN|ADMIN_TOKEN|API_TOKEN|WEBHOOK_URL|CLIENT_SECRET|SESSION_SECRET)"
        assignment = sensitive_key "[[:space:]]*[:=][[:space:]]*[\"'\'' ]?[^[:space:]\"'\'']{6,}"
        private_key = "-----BEGIN (RSA |OPENSSH |EC |DSA |PGP )?PRIVATE KEY-----"
      }
      function placeholder(line) {
        return line ~ /(replace|placeholder|example|your_|your-|<[^>]+>|xxx|random_string|change_me|never commit|set .* in \.env|\$\{[^}]+\})/
      }
      function allowed_template(file, line) {
        return file == "conf/app.conf" || file == "docker-compose.yml" || file ~ /^\.github\/workflows\//
      }
      function allowed_test(file) {
        return file ~ /_test\.(go|js|ts)$/
      }
      function allowed_checker_rule(file, line) {
        return file == "scripts/check-secret-leaks.sh" && line ~ /(pg_kv_password|sensitive_key|allowed_template)/
      }
      function allowed(file, line) {
        return allowed_template(file, line) || allowed_test(file) || allowed_checker_rule(file, line)
      }
      {
        line = $0
        if (line ~ /^[[:space:]]*#/) next
        if (allowed(FILENAME, line)) next
        kind = ""
        if (line ~ credential_url && !placeholder(line)) kind = "credential-url"
        else if (line ~ /dataSourceName[[:space:]]*=/ && line ~ pg_kv_password && !placeholder(line)) kind = "postgres-password"
        else if (line ~ assignment && !placeholder(line)) kind = "secret-assignment"
        else if (line ~ private_key) kind = "private-key"
        if (kind != "") {
          redacted = line
          gsub(/:\/\/[^[:space:]\/:@]+:[^[:space:]@]+@/, "://***:***@", redacted)
          gsub(/password=[^[:space:]]+/, "password=***", redacted)
          gsub(/(dataSourceName|redisEndpoint|radiusSecret|SQL_DSN|DATABASE_URL|POSTGRES_PASSWORD|PGPASSWORD|DB_PASSWORD|ACCESS_TOKEN|ADMIN_TOKEN|API_TOKEN|WEBHOOK_URL|CLIENT_SECRET|SESSION_SECRET)[[:space:]]*[:=].*/, "\\1 = ***", redacted)
          printf "%s:%d:%s:%s\n", FILENAME, FNR, kind, redacted
        }
      }
    ' > "$tmp_file" || true

if [ -s "$tmp_file" ]; then
  echo "Potential committed secret material found:" >&2
  cat "$tmp_file" >&2
  exit 1
fi

echo "No obvious secret leaks found."
