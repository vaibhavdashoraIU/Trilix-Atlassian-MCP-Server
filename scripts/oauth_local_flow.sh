#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3000}"
REDIRECT_URI="${REDIRECT_URI:-}"
SCOPE="${SCOPE:-mcp:tools}"
DCR_TOKEN="${DCR_TOKEN:-${OAUTH_DCR_ACCESS_TOKEN:-}}"

if [[ -z "$DCR_TOKEN" ]]; then
  echo "DCR_TOKEN or OAUTH_DCR_ACCESS_TOKEN must be set."
  exit 1
fi

if [[ -z "$REDIRECT_URI" ]]; then
  redirect_port="$(
    python3 - <<'PY'
import socket
sock = socket.socket()
sock.bind(("127.0.0.1", 0))
port = sock.getsockname()[1]
sock.close()
print(port)
PY
  )"
  REDIRECT_URI="http://127.0.0.1:${redirect_port}/callback"
fi

echo "Using redirect URI: ${REDIRECT_URI}"

echo "Registering client..."
register_resp="$(mktemp)"
register_code="$(
  curl -sS -o "${register_resp}" -w "%{http_code}" -X POST "${BASE_URL}/oauth/register" \
    -H "Authorization: Bearer ${DCR_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
      \"redirect_uris\": [\"${REDIRECT_URI}\"],
      \"client_name\": \"Local Test Client\",
      \"token_endpoint_auth_method\": \"none\"
    }"
)"

if [[ "$register_code" != "201" ]]; then
  echo "Client registration failed (HTTP ${register_code}). Response:"
  cat "${register_resp}"
  rm -f "${register_resp}"
  exit 1
fi

client_id="$(
  python3 -c 'import json,sys; print(json.load(sys.stdin).get("client_id",""))' < "${register_resp}"
)"

if [[ -z "$client_id" ]]; then
  echo "Failed to register client. Response:"
  cat "${register_resp}"
  rm -f "${register_resp}"
  exit 1
fi
rm -f "${register_resp}"

echo "Client ID: $client_id"

verifier="$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')"
challenge="$(printf "%s" "$verifier" | openssl dgst -sha256 -binary | openssl base64 | tr '+/' '-_' | tr -d '=')"

code_file="$(mktemp)"

python3 - <<'PY' "$REDIRECT_URI" "$code_file" &
import http.server
import urllib.parse
import sys

redirect_uri = sys.argv[1]
code_file = sys.argv[2]
parsed = urllib.parse.urlparse(redirect_uri)
host = parsed.hostname or "127.0.0.1"
port = parsed.port or (443 if parsed.scheme == "https" else 80)

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        query = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
        code = query.get("code", [""])[0]
        if code:
            with open(code_file, "w") as fh:
                fh.write(code)
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.end_headers()
            self.wfile.write(b"<h1>Authorization complete.</h1>You can close this window.")
        else:
            self.send_response(400)
            self.end_headers()
    def log_message(self, fmt, *args):
        return

server = http.server.HTTPServer((host, port), Handler)
server.handle_request()
PY

auth_url="${BASE_URL}/oauth/authorize?response_type=code&client_id=${client_id}&redirect_uri=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${REDIRECT_URI}', safe=''))")&scope=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${SCOPE}', safe=''))")&code_challenge=${challenge}&code_challenge_method=S256&state=local"

echo "Open this URL to sign in:"
echo "$auth_url"

echo "Waiting for authorization code..."
for _ in {1..60}; do
  if [[ -s "$code_file" ]]; then
    break
  fi
  sleep 2
done

if [[ ! -s "$code_file" ]]; then
  echo "Timed out waiting for authorization code."
  exit 1
fi

code="$(cat "$code_file")"
rm -f "$code_file"

token_resp="$(
  curl -s -X POST "${BASE_URL}/oauth/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=authorization_code&client_id=${client_id}&code=${code}&redirect_uri=${REDIRECT_URI}&code_verifier=${verifier}"
)"

access_token="$(
  python3 -c 'import json,sys; print(json.load(sys.stdin).get("access_token",""))' <<< "$token_resp"
)"

if [[ -z "$access_token" ]]; then
  echo "Token exchange failed. Response:"
  echo "$token_resp"
  exit 1
fi

echo "Access token issued."

echo "Calling /api/workspaces..."
curl -s "${BASE_URL}/api/workspaces" -H "Authorization: Bearer ${access_token}"
echo
