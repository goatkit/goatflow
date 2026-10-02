"""HTTP transport for the GoatFlow SDK."""

import json
from typing import Any, Dict, Mapping, Optional, Tuple

import httpx

from .auth import Authenticator, NoAuth
from .exceptions import (
    ForbiddenError,
    GoatflowError,
    NetworkError,
    NotFoundError,
    RateLimitError,
    ServerError,
    TimeoutError,
    UnauthorizedError,
)

_MISSING = object()


def decode_response(
    status: int, text: str, retry_after: Optional[str] = None
) -> Tuple[Any, Optional[Dict[str, Any]]]:
    """Turn an API response into ``(payload, pagination)`` or raise.

    GoatFlow wraps most responses in ``{"success": bool, "data": ..., "error": ...}``
    (paginated lists add ``"pagination"``). Some endpoints answer
    ``{"success": true, ...fields}`` without ``"data"``, and some send a bare
    object; both are returned whole. Errors are ``{"error": "message"}`` or
    ``{"error": {"code": "...", "message": "..."}}``, with or without ``"success"``.
    """
    stripped = text.strip()
    parsed: Any = _MISSING
    if stripped:
        try:
            parsed = json.loads(stripped)
        except ValueError:
            pass
    fields: Dict[str, Any] = parsed if isinstance(parsed, dict) else {}
    success = fields.get("success")
    has_success = isinstance(success, bool)

    if not 200 <= status <= 299 or (has_success and success is False):
        raise _api_error(status, stripped, fields, parsed is not _MISSING, retry_after)
    if not stripped:
        return None, None
    if parsed is _MISSING:
        raise GoatflowError(f"Response is not JSON (HTTP {status})", status, body=stripped)

    pagination = fields.get("pagination")
    if not isinstance(pagination, dict):
        pagination = None
    if has_success and "data" in fields:
        return fields["data"], pagination
    return parsed, pagination


def _api_error(
    status: int,
    body: str,
    fields: Dict[str, Any],
    is_json: bool,
    retry_after: Optional[str],
) -> GoatflowError:
    message = ""
    code: Optional[str] = None
    error = fields.get("error")
    if isinstance(error, str):
        message = error
    elif isinstance(error, dict):
        if isinstance(error.get("message"), str):
            message = error["message"]
        if isinstance(error.get("code"), str):
            code = error["code"]
    if not message and isinstance(fields.get("message"), str):
        message = fields["message"]
    if not message and not 200 <= status <= 299:
        message = httpx.codes.get_reason_phrase(status) or f"HTTP {status}"
    message = message or "request failed"
    raw = None if is_json else body

    if status == 401:
        return UnauthorizedError(message, status, code, raw)
    if status == 403:
        return ForbiddenError(message, status, code, raw)
    if status == 404:
        return NotFoundError(message, status, code, raw)
    if status == 429:
        seconds = int(retry_after) if retry_after and retry_after.isdigit() else None
        return RateLimitError(message, status, code, raw, retry_after=seconds)
    if 500 <= status <= 599:
        return ServerError(message, status, code, raw)
    return GoatflowError(message, status, code, raw)


class HTTPClient:
    """Sends requests to the GoatFlow API and decodes the responses."""

    def __init__(
        self,
        base_url: str,
        auth: Optional[Authenticator] = None,
        timeout: float = 30.0,
        user_agent: str = "goatflow-python-sdk/1.0.0",
        transport: Optional[httpx.AsyncBaseTransport] = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.auth: Authenticator = auth or NoAuth()
        self.timeout = timeout
        # Unauthenticated calls may be redirected to the login page; surface
        # the redirect instead of following it to HTML.
        self._client = httpx.AsyncClient(
            base_url=self.base_url,
            timeout=httpx.Timeout(timeout),
            headers={"User-Agent": user_agent, "Accept": "application/json"},
            follow_redirects=False,
            transport=transport,
        )

    async def close(self) -> None:
        await self._client.aclose()

    async def request(
        self,
        method: str,
        path: str,
        params: Optional[Mapping[str, Any]] = None,
        json_body: Any = None,
        authenticate: bool = True,
    ) -> Tuple[Any, Optional[Dict[str, Any]]]:
        """Send a request; return the decoded payload and the pagination, if any.

        With ``authenticate=False`` no credentials are sent (login and refresh).
        """
        headers = await self.auth.headers() if authenticate else {}
        query = {
            key: ",".join(str(v) for v in value) if isinstance(value, (list, tuple)) else _query_value(value)
            for key, value in (params or {}).items()
            if value is not None and value is not False and value != "" and value != []
        }
        try:
            response = await self._client.request(
                method, path, params=query, json=json_body, headers=headers
            )
        except httpx.TimeoutException as exc:
            raise TimeoutError(f"{method} {self.base_url}{path} timed out after {self.timeout}s") from exc
        except httpx.HTTPError as exc:
            raise NetworkError(f"{method} {self.base_url}{path}: {exc}") from exc
        return decode_response(response.status_code, response.text, response.headers.get("Retry-After"))

    async def get(self, path: str, params: Optional[Mapping[str, Any]] = None) -> Any:
        return (await self.request("GET", path, params=params))[0]

    async def post(self, path: str, json_body: Any = None) -> Any:
        return (await self.request("POST", path, json_body=json_body))[0]

    async def put(self, path: str, json_body: Any = None) -> Any:
        return (await self.request("PUT", path, json_body=json_body))[0]

    async def delete(self, path: str) -> Any:
        return (await self.request("DELETE", path))[0]


def _query_value(value: Any) -> str:
    if value is True:
        return "true"
    return str(value)
