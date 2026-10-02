"""Tests against response bodies shared with the Go and TypeScript SDKs.

sdk/testdata holds bodies captured from a running server (GET endpoints) or
the handler's JSON literal (endpoints that change data).
"""

import asyncio
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import httpx
import pytest

from goatflow_sdk import (
    ArticleCreateRequest,
    GoatflowClient,
    GoatflowError,
    NetworkError,
    NotFoundError,
    SearchQuery,
    TicketCreateRequest,
    UnauthorizedError,
)

TESTDATA = Path(__file__).resolve().parents[2] / "testdata"


def fixture(name: str) -> str:
    return (TESTDATA / f"{name}.json").read_text()


class Recorder:
    def __init__(self) -> None:
        self.request: Optional[httpx.Request] = None

    @property
    def body(self) -> Any:
        assert self.request is not None
        return json.loads(self.request.content) if self.request.content else None


def serve(status: int, body: str, headers: Optional[Dict[str, str]] = None) -> Tuple[GoatflowClient, Recorder]:
    """Client whose every request is answered with status and body."""
    rec = Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        rec.request = request
        return httpx.Response(
            status,
            content=body.encode(),
            headers={"Content-Type": "application/json; charset=utf-8", **(headers or {})},
        )

    client = GoatflowClient.with_api_key("http://goatflow.test", "gf_test_token", transport=httpx.MockTransport(handler))
    return client, rec


def run(coro: Any) -> Any:
    return asyncio.run(coro)


def test_tickets_list_unwraps_data_and_pagination() -> None:
    client, rec = serve(200, fixture("ticket_list"))

    result = run(client.tickets.list(per_page=2, status="open", queue_id=37, include=["article_count", "last_article"]))

    assert rec.request is not None
    assert rec.request.method == "GET"
    assert rec.request.url.path == "/api/v1/tickets"
    assert dict(rec.request.url.params) == {
        "per_page": "2",
        "status": "open",
        "queue_id": "37",
        "include": "article_count,last_article",
    }
    assert rec.request.headers["Authorization"] == "Bearer gf_test_token"
    assert len(result.tickets) == 1
    ticket = result.tickets[0]
    assert (ticket.id, ticket.ticket_number, ticket.state_name, ticket.responsible_user_id) == (
        37558,
        "COACH-9664946",
        "new",
        14,
    )
    assert ticket.created_at == datetime(2026, 9, 1, 15, 23, 18, tzinfo=timezone.utc)
    assert result.pagination.model_dump() == {
        "page": 1,
        "per_page": 2,
        "total": 37445,
        "total_pages": 18723,
        "has_next": True,
        "has_prev": False,
    }


def test_tickets_get_unwraps_data() -> None:
    client, rec = serve(200, fixture("ticket_get"))

    ticket = run(client.tickets.get(37558))

    assert rec.request is not None and rec.request.url.path == "/api/v1/tickets/37558"
    assert (ticket.id, ticket.title, ticket.state, ticket.queue, ticket.article_count) == (
        37558,
        "E2E identity check",
        "new",
        "Coaching",
        1,
    )
    assert ticket.type_id is None


def test_error_envelope_raises_typed_error_with_status() -> None:
    client, _ = serve(404, fixture("ticket_not_found"))

    with pytest.raises(NotFoundError) as excinfo:
        run(client.tickets.get(999999999))

    assert (excinfo.value.status_code, excinfo.value.message, excinfo.value.code) == (404, "Ticket not found", None)


def test_structured_auth_error_keeps_code() -> None:
    client, _ = serve(401, fixture("invalid_token"))

    with pytest.raises(UnauthorizedError) as excinfo:
        run(client.users.me())

    assert (excinfo.value.status_code, excinfo.value.code, excinfo.value.message) == (
        401,
        "core:invalid_token",
        "Invalid or malformed token",
    )


def test_success_false_with_200_is_an_error() -> None:
    client, _ = serve(200, '{"success":false,"error":"Database unavailable"}')

    with pytest.raises(GoatflowError) as excinfo:
        run(client.queues.get(1))

    assert (excinfo.value.status_code, excinfo.value.message) == (200, "Database unavailable")


def test_redirect_is_not_followed() -> None:
    client, _ = serve(303, '<a href="/login">See Other</a>.', {"Location": "/login", "Content-Type": "text/html"})

    with pytest.raises(GoatflowError) as excinfo:
        run(client.tickets.list())

    assert (excinfo.value.status_code, excinfo.value.message, excinfo.value.body) == (
        303,
        "See Other",
        '<a href="/login">See Other</a>.',
    )


def test_bare_article_list_is_returned_whole() -> None:
    client, rec = serve(200, fixture("article_list"))

    result = run(client.articles.list(37558, include_attachments=True))

    assert rec.request is not None
    assert rec.request.url.path == "/api/v1/tickets/37558/articles"
    assert dict(rec.request.url.params) == {"include_attachments": "true"}
    assert result.total == 1
    article = result.articles[0]
    assert (article.id, article.article_type, article.is_visible_for_customer, article.from_) == (
        91,
        "note-internal",
        False,
        "",
    )
    assert article.attachments is not None and article.attachments[0].filename == "log.txt"


def test_reopen_reads_top_level_fields() -> None:
    client, rec = serve(200, fixture("reopen"))

    result = run(client.tickets.reopen(37558, "Customer replied"))

    assert rec.request is not None and rec.request.url.path == "/api/v1/tickets/37558/reopen"
    assert rec.body == {"reason": "Customer replied"}
    assert (result.id, result.state_id, result.state) == (37558, 4, "open")


def test_delete_accepts_204() -> None:
    client, rec = serve(204, "")

    assert run(client.tickets.delete(37558)) is None
    assert rec.request is not None and rec.request.method == "DELETE"


def test_create_ticket_sends_only_set_fields() -> None:
    client, rec = serve(201, fixture("ticket_created"))

    created = run(client.tickets.create(TicketCreateRequest(title="Printer on fire", queue_id=37, body="Smoke everywhere")))

    assert rec.request is not None and rec.request.method == "POST"
    assert rec.request.headers["Content-Type"] == "application/json"
    assert rec.body == {"title": "Printer on fire", "queue_id": 37, "body": "Smoke everywhere"}
    assert (created.id, created.tn, created.ticket_state_id) == (37559, "2026100110000017", 1)


def test_article_create_sends_from_alias() -> None:
    client, rec = serve(201, '{"success":false,"error":"Article body is required"}')

    with pytest.raises(GoatflowError):
        run(client.articles.create(37558, ArticleCreateRequest(body="x", from_="agent@example.com")))

    assert rec.body == {"body": "x", "from": "agent@example.com"}


def test_users_me_and_queue_get() -> None:
    client, rec = serve(200, fixture("user_me"))
    me = run(client.users.me())
    assert rec.request is not None and rec.request.url.path == "/api/v1/users/me"
    assert (me.id, me.login, me.active, [g.name for g in me.groups]) == (1, "root@localhost", True, ["users", "admin"])

    client, rec = serve(200, fixture("queue_get"))
    queue = run(client.queues.get(37))
    assert rec.request is not None and rec.request.url.path == "/api/v1/queues/37"
    assert (queue.name, queue.comments, queue.signature_id, queue.valid) == (
        "Coaching",
        "Coaching engagement queue (GoatCoach)",
        1,
        None,
    )


def test_statistics_dashboard() -> None:
    client, rec = serve(200, fixture("dashboard"))

    stats = run(client.statistics.dashboard())

    assert rec.request is not None and rec.request.url.path == "/api/v1/statistics/dashboard"
    assert stats.overview.total_tickets == 12
    assert (stats.by_queue[0].queue_name, stats.by_queue[0].count) == ("Coaching", 9)
    assert stats.recent_activity[0].ticket_tn == "COACH-9664946"


def test_search_query() -> None:
    client, rec = serve(200, fixture("search_unavailable"))

    results = run(client.search.query(SearchQuery(query="printer", limit=5)))

    assert rec.request is not None and (rec.request.method, rec.request.url.path) == ("POST", "/api/v1/search")
    assert rec.body == {"query": "printer", "limit": 5}
    assert (results.total_hits, results.hits, results.warning) == (0, [], "search backend unavailable")


def test_login_switches_to_access_token() -> None:
    client, rec = serve(200, fixture("login"))
    client.set_auth(None)

    async def scenario() -> Tuple[Any, Optional[str], Optional[str]]:
        response = await client.login("root@localhost", "secret")
        assert rec.request is not None
        login_auth = rec.request.headers.get("Authorization")
        try:
            await client.users.me()
        except GoatflowError:
            pass  # the fixture is a login body; only the header matters here
        return response, login_auth, rec.request.headers.get("Authorization")

    response, login_auth, next_auth = run(scenario())

    assert login_auth is None
    assert response.user.role == "Admin"
    assert (response.expires_in, response.refresh_expires_in) == (86400, 604800)
    assert next_auth == "Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig"


def test_failed_login_is_unauthorized() -> None:
    client, _ = serve(401, fixture("login_failed"))

    with pytest.raises(UnauthorizedError) as excinfo:
        run(client.auth.login("root@localhost", "wrong"))

    assert excinfo.value.message == "Invalid credentials"


def test_refresh_posts_refresh_token_without_credentials() -> None:
    client, rec = serve(200, fixture("refresh"))

    pair = run(client.auth.refresh("refresh-1"))

    assert rec.request is not None
    assert (rec.request.method, rec.request.url.path) == ("POST", "/api/v1/auth/refresh")
    assert "Authorization" not in rec.request.headers
    assert rec.body == {"refresh_token": "refresh-1"}
    assert (pair.access_token, pair.refresh_token, pair.expires_in) == (
        "eyJhbGciOiJIUzI1NiJ9.e30.sig2",
        "eyJhbGciOiJIUzI1NiJ9.e30.ref2",
        86400,
    )


def test_rejected_refresh_token_is_unauthorized() -> None:
    client, _ = serve(401, fixture("refresh_rejected"))

    with pytest.raises(UnauthorizedError) as excinfo:
        run(client.auth.refresh("stale"))

    assert excinfo.value.message == "Invalid or expired refresh token"


def serve_routes(routes: Dict[str, Tuple[int, str]]) -> Tuple[httpx.MockTransport, List[httpx.Request]]:
    """Transport answering by path; returns it and the log of every request."""
    log: List[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        log.append(request)
        status, body = routes.get(request.url.path, (404, '{"error":"no such route in test"}'))
        return httpx.Response(status, content=body.encode(), headers={"Content-Type": "application/json"})

    return httpx.MockTransport(handler), log


def test_expired_jwt_is_renewed_through_refresh_endpoint() -> None:
    transport, log = serve_routes(
        {
            "/api/v1/auth/refresh": (200, fixture("refresh")),
            "/api/v1/users/me": (200, fixture("user_me")),
        }
    )
    expired = datetime.now(timezone.utc) - timedelta(hours=1)
    client = GoatflowClient.with_jwt("http://goatflow.test", "expired", "refresh-1", expired, transport=transport)

    async def scenario() -> None:
        await asyncio.gather(client.users.me(), client.users.me())
        await client.users.me()

    run(scenario())

    assert [r.url.path for r in log] == ["/api/v1/auth/refresh"] + ["/api/v1/users/me"] * 3
    assert "Authorization" not in log[0].headers
    assert json.loads(log[0].content) == {"refresh_token": "refresh-1"}
    assert {r.headers["Authorization"] for r in log[1:]} == {"Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig2"}


def test_rejected_refresh_fails_the_request() -> None:
    transport, log = serve_routes({"/api/v1/auth/refresh": (401, fixture("refresh_rejected"))})
    expired = datetime.now(timezone.utc) - timedelta(hours=1)
    client = GoatflowClient.with_jwt("http://goatflow.test", "expired", "refresh-1", expired, transport=transport)

    with pytest.raises(GoatflowError):
        run(client.users.me())

    assert [r.url.path for r in log] == ["/api/v1/auth/refresh"]


def test_transport_failure_is_network_error() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused", request=request)

    client = GoatflowClient.with_api_key("http://goatflow.test", "gf_x", transport=httpx.MockTransport(handler))

    with pytest.raises(NetworkError) as excinfo:
        run(client.tickets.get(1))

    assert "GET http://goatflow.test/api/v1/tickets/1" in excinfo.value.message
