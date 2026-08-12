"""Watermelon UI registry availability gate.

The grilled adoption decision (.scratch/watermelon-ui-adoption/spec.md)
deferred the Watermelon migration until the registry serves real component
manifests. This gate operationalizes that condition: it requests the
registry manifest endpoint and passes only when the response is JSON with
the expected manifest shape. An HTML response (the SPA fallback observed
when the decision was made), an error status, or a timeout fails the gate.

Exit codes: 0 = ready, 1 = not ready, 2 = usage error.
"""

from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request

DEFAULT_MANIFEST_URL = "https://registry.watermelon.sh/r/button.json"


def check_registry(url: str, timeout: float = 10.0) -> bool:
    """Returns True only when url serves a JSON registry manifest."""
    try:
        request = urllib.request.Request(url, headers={"User-Agent": "shadcn"})
        with urllib.request.urlopen(request, timeout=timeout) as response:
            content_type = response.headers.get("Content-Type", "")
            if "json" not in content_type.lower():
                return False
            body = response.read()
            data = json.loads(body)
    except (
        urllib.error.URLError,
        urllib.error.HTTPError,
        json.JSONDecodeError,
        TimeoutError,
        OSError,
    ):
        return False
    if not isinstance(data, dict):
        return False
    # A shadcn-style registry manifest carries a component name and a file
    # list; any JSON object with a name is treated as a manifest.
    return bool(data.get("name")) and isinstance(data.get("files"), list)


def main(argv: list[str] | None = None) -> int:
    argv = argv if argv is not None else sys.argv[1:]
    url = argv[0] if argv else DEFAULT_MANIFEST_URL
    ready = check_registry(url)
    print(f"watermelon registry: {'READY' if ready else 'NOT READY'} ({url})")
    return 0 if ready else 1


if __name__ == "__main__":
    raise SystemExit(main())
