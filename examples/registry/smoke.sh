#!/usr/bin/env bash
# Synthetic, isolated, process-level smoke. Run from repository root.
# Usage: bash examples/registry/smoke.sh /absolute/path/to/nebprx [new-artifact-dir]
set -euo pipefail
bin="$(realpath "${1:?binary required}")"
work="${2:-$(mktemp -d /tmp/prx-registry-smoke.XXXXXX)}"
mkdir -p "$work"
chmod 700 "$work"
work="$(realpath "$work")"
exec > >(tee "$work/smoke.log") 2>&1
server_pid= client_pid=
cleanup() {
  [ -z "$client_pid" ] || kill "$client_pid" 2>/dev/null || true
  [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true
  [ -z "$client_pid" ] || wait "$client_pid" 2>/dev/null || true
  [ -z "$server_pid" ] || wait "$server_pid" 2>/dev/null || true
}
trap cleanup EXIT
server_port="${PRX_SMOKE_SERVER_PORT:-18091}"
client_port="${PRX_SMOKE_CLIENT_PORT:-18092}"
server=("$bin" registry --mode server --state-dir "$work/server")
client=("$bin" registry --mode client --state-dir "$work/client")
"${server[@]}" --action init > "$work/server-init.json"
python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["public_key"])' "$work/server-init.json" > "$work/server.pub"
"${server[@]}" --action enroll --installation smoke --output "$work/install.token"
for ecosystem in npm PyPI crates.io Go; do
  input=examples/registry/empty.json
  [ "$ecosystem" != npm ] || input=examples/registry/npm.json
  "${server[@]}" --action import --ecosystem "$ecosystem" --input "$input" --observed-at "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
done
install -m 600 examples/registry/inventory.json "$work/inventory.json"
"${client[@]}" --action inventory --input "$work/inventory.json"
remote=(--upstream "http://127.0.0.1:$server_port" --installation smoke --token-file "$work/install.token" --trust-key "$work/server.pub" --allow-loopback-http)
start_server() {
  "${server[@]}" --listen "127.0.0.1:$server_port" --allow-loopback-http >> "$work/server.log" 2>&1 & server_pid=$!
}
wait_status() {
  python3 - "$1" "$2" <<'PY'
import sys,time,urllib.request
url,path=sys.argv[1:]
for _ in range(100):
    try:
        token=open(path).read().strip()
        request=urllib.request.Request(url,headers={'Authorization':'Bearer '+token})
        with urllib.request.urlopen(request,timeout=1) as response:
            assert response.status==200
        break
    except (OSError,AssertionError): time.sleep(.1)
else: raise SystemExit('service did not become ready')
PY
}
start_server
wait_status "http://127.0.0.1:$server_port/v1/status" "$work/server/local.token"
"${client[@]}" --action sync "${remote[@]}"
"${client[@]}" --action check --ecosystem npm --name prx-synthetic-demo --package-version 1.0.0 --public > "$work/affected.json"
"${client[@]}" --action check --ecosystem npm --name prx-synthetic-demo --package-version 2.0.0 --public > "$work/fixed.json"
"${client[@]}" --action check --ecosystem npm --name first-seen-demo --package-version 1.0.0 --public > "$work/first-offline.json"
"${client[@]}" --action check --online --ecosystem npm --name first-seen-demo --package-version 1.0.0 --public "${remote[@]}" > "$work/first-online.json"
"${client[@]}" --listen "127.0.0.1:$client_port" --allow-loopback-http >> "$work/client.log" 2>&1 & client_pid=$!
wait_status "http://127.0.0.1:$client_port/v1/status" "$work/client/local.token"
kill "$client_pid" "$server_pid"
wait "$client_pid"; client_pid=
wait "$server_pid"; server_pid=
"${client[@]}" --action check --ecosystem npm --name prx-synthetic-demo --package-version 1.0.0 --public > "$work/restarted-check.json"
start_server
wait_status "http://127.0.0.1:$server_port/v1/status" "$work/server/local.token"
"${client[@]}" --action sync "${remote[@]}"
python3 - "$work" "$server_port" <<'PY'
import json,sys,urllib.request,urllib.error
from pathlib import Path
work=Path(sys.argv[1]); port=sys.argv[2]
for file,expected in [('affected.json','known_affected'),('fixed.json','no_known_match'),('first-offline.json','indeterminate'),('first-online.json','no_known_match'),('restarted-check.json','known_affected')]:
    data=json.loads((work/file).read_text()); assert data['finding']==expected,(file,data)
payload=json.dumps({'installation':'smoke','packages':[]}).encode()
req=urllib.request.Request('http://127.0.0.1:'+port+'/v1/snapshot',data=payload,headers={'Authorization':'Bearer '+'z'*43,'Content-Type':'application/json'})
try: urllib.request.urlopen(req); raise AssertionError('wrong token accepted')
except urllib.error.HTTPError as error: assert error.code==401
print('PASS: affected/fixed, online/offline admission, client/server restart, invalid token')
PY
"${client[@]}" --action status > "$work/final-status.json"
echo "Artifacts: $work"
