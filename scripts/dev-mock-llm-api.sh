#!/usr/bin/env bash
set -euo pipefail

addr="127.0.0.1:18082"

usage() {
  cat <<'EOF'
Usage: scripts/dev-mock-llm-api.sh [--addr HOST:PORT]

Runs a local OpenAI-compatible mock LLM API and verdict endpoint for KiwiGuard
development. The server exposes:

  POST /v1/chat/completions
  POST /v1/responses
  POST /verdict
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --addr)
      addr="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$addr" || "$addr" != *:* ]]; then
  echo "--addr must use HOST:PORT" >&2
  exit 2
fi

host="${addr%:*}"
port="${addr##*:}"

python3 - "$host" "$port" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


host = sys.argv[1]
port = int(sys.argv[2])


def text_from_chat(payload):
    parts = []
    for message in payload.get("messages", []):
        content = message.get("content", "")
        if isinstance(content, str):
            parts.append(content)
        elif isinstance(content, list):
            for item in content:
                if isinstance(item, dict) and isinstance(item.get("text"), str):
                    parts.append(item["text"])
    return "\n".join(parts)


def text_from_response(payload):
    value = payload.get("input", "")
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(str(item) for item in value)
    return str(value)


class Handler(BaseHTTPRequestHandler):
    server_version = "KiwiGuardDevMock/1.0"

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        try:
            payload = json.loads(raw or b"{}")
        except json.JSONDecodeError:
            self.send_json(400, {"error": {"message": "invalid json"}})
            return

        if self.path == "/v1/chat/completions":
            prompt = text_from_chat(payload)
            self.send_json(200, {
                "id": "chatcmpl-dev",
                "object": "chat.completion",
                "created": 1777796037,
                "model": payload.get("model", "mock-secure-model"),
                "choices": [{
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": f"mock llm captured input and produced a guarded response: {prompt}",
                    },
                    "finish_reason": "stop",
                }],
            })
            return

        if self.path == "/v1/responses":
            prompt = text_from_response(payload)
            self.send_json(200, {
                "id": "resp-dev",
                "object": "response",
                "model": payload.get("model", "mock-secure-model"),
                "output_text": f"mock llm response: {prompt}",
            })
            return

        if self.path == "/verdict":
            text = str(payload.get("text", ""))
            should_block = "BLOCK_ME" in text
            self.send_json(200, {
                "risk_level": "high" if should_block else "low",
                "categories": ["dev.block"] if should_block else ["dev.allow"],
                "confidence": 0.99,
                "suggested_action": "block" if should_block else "allow",
                "matched_spans": [],
                "rationale": "local development verdict",
                "provider_name": "dev-security-model",
                "fallback_used": False,
            })
            return

        self.send_json(404, {"error": {"message": "not found"}})

    def log_message(self, fmt, *args):
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    def send_json(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


print(f"KiwiGuard mock LLM API listening on http://{host}:{port}", flush=True)
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY
