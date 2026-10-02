"""GoatFlow REST API client."""

from datetime import datetime, timedelta, timezone
from typing import Any, Dict, List, Optional, Sequence, Tuple, Type, TypeVar

import httpx
from pydantic import BaseModel, ValidationError

from .auth import APIKeyAuth, Authenticator, JWTAuth, NoAuth
from .exceptions import AuthenticationError, GoatflowError, ResponseShapeError
from .http_client import HTTPClient
from .models import (
    Article,
    ArticleCreateRequest,
    ArticleList,
    ArticleUpdate,
    ArticleUpdateRequest,
    CreatedTicket,
    CreatedUser,
    CurrentUser,
    DashboardStatistics,
    Health,
    TokenPair,
    Pagination,
    Queue,
    ReopenResult,
    RequestModel,
    SearchQuery,
    SearchResults,
    Ticket,
    TicketCreateRequest,
    TicketList,
    TicketRecord,
    TicketSummary,
    TicketUpdateRequest,
    User,
    UserCreateRequest,
    UserList,
    UserUpdateRequest,
    Webhook,
    WebhookDelivery,
    WebhookRequest,
)


M = TypeVar("M", bound=BaseModel)


def _body(request: RequestModel) -> Any:
    return request.model_dump(exclude_none=True, by_alias=True)


def _parse(model: Type[M], data: Any) -> M:
    """Validate a decoded response payload; a shape mismatch is a GoatflowError."""
    try:
        return model.model_validate(data)
    except ValidationError as exc:
        raise ResponseShapeError(f"Unexpected {model.__name__} response: {exc}") from exc


def _pagination(pagination: Optional[dict], what: str) -> Pagination:
    if pagination is None:
        raise ResponseShapeError(f"{what} response has no pagination")
    return _parse(Pagination, pagination)


class TicketsService:
    """/api/v1/tickets"""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def list(
        self,
        *,
        page: Optional[int] = None,
        per_page: Optional[int] = None,
        status: Optional[str] = None,
        queue_id: Optional[int] = None,
        priority_id: Optional[int] = None,
        customer_user_id: Optional[str] = None,
        assigned_user_id: Optional[int] = None,
        search: Optional[str] = None,
        sort: Optional[str] = None,
        order: Optional[str] = None,
        include: Optional[Sequence[str]] = None,
    ) -> TicketList:
        """One page of tickets the caller can read.

        ``status`` is open, closed, pending or an exact state name; ``sort`` is
        created, updated, priority, tn or title; ``include`` may contain
        article_count and last_article.
        """
        params = {
            "page": page,
            "per_page": per_page,
            "status": status,
            "queue_id": queue_id,
            "priority_id": priority_id,
            "customer_user_id": customer_user_id,
            "assigned_user_id": assigned_user_id,
            "search": search,
            "sort": sort,
            "order": order,
            "include": list(include) if include else None,
        }
        data, pagination = await self._http.request("GET", "/api/v1/tickets", params=params)
        return TicketList(
            tickets=[_parse(TicketSummary, item) for item in data or []],
            pagination=_pagination(pagination, "Ticket list"),
        )

    async def get(self, ticket_id: int) -> Ticket:
        return _parse(Ticket, await self._http.get(f"/api/v1/tickets/{ticket_id}"))

    async def create(self, request: TicketCreateRequest) -> CreatedTicket:
        return _parse(CreatedTicket, await self._http.post("/api/v1/tickets", _body(request)))

    async def update(self, ticket_id: int, request: TicketUpdateRequest) -> TicketRecord:
        """Change the set fields and return the updated row."""
        data = await self._http.put(f"/api/v1/tickets/{ticket_id}", _body(request))
        return _parse(TicketRecord, data)

    async def delete(self, ticket_id: int) -> None:
        await self._http.delete(f"/api/v1/tickets/{ticket_id}")

    async def reopen(self, ticket_id: int, reason: str) -> ReopenResult:
        """Move a closed ticket back to open and record the reason as an article."""
        data = await self._http.post(f"/api/v1/tickets/{ticket_id}/reopen", {"reason": reason})
        return _parse(ReopenResult, data)


class ArticlesService:
    """/api/v1/tickets/{id}/articles"""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def list(self, ticket_id: int, include_attachments: bool = False) -> ArticleList:
        """A ticket's articles, newest first."""
        data = await self._http.get(
            f"/api/v1/tickets/{ticket_id}/articles", {"include_attachments": include_attachments}
        )
        return _parse(ArticleList, data)

    async def get(self, ticket_id: int, article_id: int, include_attachments: bool = False) -> Article:
        data = await self._http.get(
            f"/api/v1/tickets/{ticket_id}/articles/{article_id}",
            {"include_attachments": include_attachments},
        )
        return _parse(Article, data)

    async def create(self, ticket_id: int, request: ArticleCreateRequest) -> Article:
        data = await self._http.post(f"/api/v1/tickets/{ticket_id}/articles", _body(request))
        return _parse(Article, data)

    async def update(self, ticket_id: int, article_id: int, request: ArticleUpdateRequest) -> ArticleUpdate:
        data = await self._http.put(f"/api/v1/tickets/{ticket_id}/articles/{article_id}", _body(request))
        return _parse(ArticleUpdate, data)

    async def delete(self, ticket_id: int, article_id: int) -> None:
        """Delete an article and its attachments."""
        await self._http.delete(f"/api/v1/tickets/{ticket_id}/articles/{article_id}")


class UsersService:
    """/api/v1/users (agents)"""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def list(
        self,
        *,
        page: Optional[int] = None,
        per_page: Optional[int] = None,
        search: Optional[str] = None,
        valid: Optional[str] = None,
        group_id: Optional[int] = None,
    ) -> UserList:
        """One page of agents; ``valid`` is "1" (valid only) or "2" (invalid only)."""
        params = {"page": page, "per_page": per_page, "search": search, "valid": valid, "group_id": group_id}
        data, pagination = await self._http.request("GET", "/api/v1/users", params=params)
        return UserList(
            users=[_parse(User, item) for item in data or []],
            pagination=_pagination(pagination, "User list"),
        )

    async def get(self, user_id: int) -> User:
        """One agent, including email and preferences."""
        return _parse(User, await self._http.get(f"/api/v1/users/{user_id}"))

    async def me(self) -> CurrentUser:
        """The authenticated agent."""
        return _parse(CurrentUser, await self._http.get("/api/v1/users/me"))

    async def create(self, request: UserCreateRequest) -> CreatedUser:
        return _parse(CreatedUser, await self._http.post("/api/v1/users", _body(request)))

    async def update(self, user_id: int, request: UserUpdateRequest) -> None:
        """Change the set fields. The API answers only with the id; use get() to read the result."""
        await self._http.put(f"/api/v1/users/{user_id}", _body(request))

    async def delete(self, user_id: int) -> None:
        await self._http.delete(f"/api/v1/users/{user_id}")


class QueuesService:
    """/api/v1/queues"""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def list(self, *, valid: Optional[str] = None, include_stats: bool = False) -> List[Queue]:
        """Queues the caller can read."""
        data = await self._http.get("/api/v1/queues", {"valid": valid, "include_stats": include_stats})
        return [_parse(Queue, item) for item in data or []]

    async def get(self, queue_id: int) -> Queue:
        return _parse(Queue, await self._http.get(f"/api/v1/queues/{queue_id}"))


class StatisticsService:
    """/api/v1/statistics"""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def dashboard(self) -> DashboardStatistics:
        """Ticket counts overall, per queue and per priority, and the newest tickets."""
        return _parse(DashboardStatistics, await self._http.get("/api/v1/statistics/dashboard"))


class SearchService:
    """POST /api/v1/search"""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def query(self, query: SearchQuery) -> SearchResults:
        return _parse(SearchResults, await self._http.post("/api/v1/search", _body(query)))


class WebhooksService:
    """/api/v1/webhooks (admin only)"""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def list(self, active: Optional[bool] = None) -> List[Webhook]:
        params = {} if active is None else {"active": "true" if active else "false"}
        data = await self._http.get("/api/v1/webhooks", params)
        return [_parse(Webhook, item) for item in data or []]

    async def get(self, webhook_id: int) -> Webhook:
        return _parse(Webhook, await self._http.get(f"/api/v1/webhooks/{webhook_id}"))

    async def create(self, request: WebhookRequest) -> Webhook:
        return _parse(Webhook, await self._http.post("/api/v1/webhooks", _body(request)))

    async def update(self, webhook_id: int, request: WebhookRequest) -> Webhook:
        data = await self._http.put(f"/api/v1/webhooks/{webhook_id}", _body(request))
        return _parse(Webhook, data)

    async def delete(self, webhook_id: int) -> None:
        await self._http.delete(f"/api/v1/webhooks/{webhook_id}")

    async def test(self, webhook_id: int) -> WebhookDelivery:
        """Send a webhook.test event now and return the recorded delivery."""
        return _parse(WebhookDelivery, await self._http.post(f"/api/v1/webhooks/{webhook_id}/test"))

    async def get_deliveries(self, webhook_id: int, limit: Optional[int] = None) -> List[WebhookDelivery]:
        """Newest deliveries first, without payload and response bodies."""
        data = await self._http.get(f"/api/v1/webhooks/{webhook_id}/deliveries", {"limit": limit})
        return [_parse(WebhookDelivery, item) for item in data or []]

    async def get_delivery(self, delivery_id: int) -> WebhookDelivery:
        """One delivery including payload and response body."""
        return _parse(WebhookDelivery, await self._http.get(f"/api/v1/webhooks/deliveries/{delivery_id}"))

    async def redeliver(self, delivery_id: int) -> WebhookDelivery:
        """Send a delivery's payload again and return the new delivery."""
        data = await self._http.post(f"/api/v1/webhooks/deliveries/{delivery_id}/redeliver")
        return _parse(WebhookDelivery, data)


class AuthService:
    """POST /api/v1/auth/login and /api/v1/auth/refresh, sent without the client's credentials."""

    def __init__(self, http: HTTPClient) -> None:
        self._http = http

    async def login(self, login: str, password: str) -> TokenPair:
        """Exchange an agent's login and password for a token pair."""
        return await self._token_pair("/api/v1/auth/login", {"login": login, "password": password})

    async def refresh(self, refresh_token: str) -> TokenPair:
        """Exchange a refresh token for a new access token and a new (rotated) refresh token.

        A rejected refresh token raises UnauthorizedError.
        """
        return await self._token_pair("/api/v1/auth/refresh", {"refresh_token": refresh_token})

    async def refresh_function(
        self, refresh_token: Optional[str]
    ) -> Tuple[str, Optional[str], Optional[datetime]]:
        """A JWTAuth ``refresh_function`` backed by :meth:`refresh`."""
        if not refresh_token:
            raise AuthenticationError("Access token expired and no refresh token is available")
        pair = await self.refresh(refresh_token)
        return pair.access_token, pair.refresh_token, _expires_at(pair)

    async def _token_pair(self, path: str, body: Dict[str, str]) -> TokenPair:
        data, _ = await self._http.request("POST", path, json_body=body, authenticate=False)
        return _parse(TokenPair, data)


def _expires_at(pair: TokenPair) -> datetime:
    return datetime.now(timezone.utc) + timedelta(seconds=pair.expires_in)


class GoatflowClient:
    """GoatFlow REST API client. Use as ``async with`` or call :meth:`close`."""

    def __init__(
        self,
        base_url: str,
        auth: Optional[Authenticator] = None,
        timeout: float = 30.0,
        user_agent: str = "goatflow-python-sdk/1.0.0",
        transport: Optional[httpx.AsyncBaseTransport] = None,
    ) -> None:
        self.http = HTTPClient(base_url, auth, timeout, user_agent, transport)
        self.tickets = TicketsService(self.http)
        self.articles = ArticlesService(self.http)
        self.users = UsersService(self.http)
        self.queues = QueuesService(self.http)
        self.statistics = StatisticsService(self.http)
        self.search = SearchService(self.http)
        self.webhooks = WebhooksService(self.http)
        self.auth = AuthService(self.http)

    @classmethod
    def with_api_key(cls, base_url: str, api_key: str, **kwargs: Any) -> "GoatflowClient":
        """Client authenticating with a GoatFlow API token (gf_...)."""
        return cls(base_url, APIKeyAuth(api_key), **kwargs)

    @classmethod
    def with_jwt(
        cls,
        base_url: str,
        token: str,
        refresh_token: Optional[str] = None,
        expires_at: Optional[datetime] = None,
        **kwargs: Any,
    ) -> "GoatflowClient":
        """Client authenticating with a JWT access token from POST /api/v1/auth/login.

        With ``refresh_token`` and ``expires_at`` the pair is renewed through
        POST /api/v1/auth/refresh shortly before expiry.
        """
        client = cls(base_url, **kwargs)
        client.set_auth(JWTAuth(token, refresh_token, expires_at, client.auth.refresh_function))
        return client

    def set_auth(self, auth: Optional[Authenticator]) -> None:
        self.http.auth = auth or NoAuth()

    async def health(self) -> Health:
        """GET /health. An unhealthy server answers 503, raised as ServerError."""
        return _parse(Health, await self.http.get("/health"))

    async def ping(self) -> bool:
        """True when the server reports itself healthy."""
        try:
            await self.health()
        except GoatflowError:
            return False
        return True

    async def login(self, login: str, password: str) -> TokenPair:
        """Log in and switch the client to the returned access token.

        The token is renewed automatically through POST /api/v1/auth/refresh.
        """
        pair = await self.auth.login(login, password)
        self.set_auth(JWTAuth(pair.access_token, pair.refresh_token, _expires_at(pair), self.auth.refresh_function))
        return pair

    async def close(self) -> None:
        await self.http.close()

    async def __aenter__(self) -> "GoatflowClient":
        return self

    async def __aexit__(self, *exc: Any) -> None:
        await self.close()
