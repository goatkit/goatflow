"""Request and response models of the GoatFlow REST API (/api/v1).

Field sets mirror what the handlers send; where two endpoints describe the
same resource differently (ticket list rows vs. a single ticket) they get
separate models. Response models keep fields the SDK does not know yet
(``extra="allow"``); request models reject unknown fields.
"""

from datetime import datetime
from typing import Any, Dict, List, Literal, Optional

from pydantic import BaseModel, ConfigDict, Field


class BaseGoatflowModel(BaseModel):
    """Base for response models."""

    model_config = ConfigDict(extra="allow", populate_by_name=True)


class RequestModel(BaseModel):
    """Base for request bodies; unset fields are not sent."""

    model_config = ConfigDict(extra="forbid", populate_by_name=True)


class Pagination(BaseGoatflowModel):
    page: int
    per_page: int
    total: int
    total_pages: int
    has_next: bool
    has_prev: bool


class GroupRef(BaseGoatflowModel):
    id: int
    name: str


# Tickets


class LastArticle(BaseGoatflowModel):
    subject: str
    created_at: datetime


class TicketSummary(BaseGoatflowModel):
    """One row of GET /api/v1/tickets."""

    id: int
    tn: str
    ticket_number: str
    title: str
    queue_id: int
    queue_name: str
    state_id: int
    state_name: str
    priority_id: int
    priority_name: str
    customer_user_id: str
    customer_id: str
    user_id: int  # owner
    responsible_user_id: Optional[int] = None
    created_at: datetime
    updated_at: datetime
    article_count: Optional[int] = None  # with include=["article_count"]
    last_article: Optional[LastArticle] = None  # with include=["last_article"]


class TicketList(BaseGoatflowModel):
    tickets: List[TicketSummary]
    pagination: Pagination


class Ticket(BaseGoatflowModel):
    """GET /api/v1/tickets/{id}."""

    id: int
    ticket_number: str
    title: str
    state_id: int
    state: str
    priority_id: int
    priority: str
    queue_id: int
    queue: str
    type_id: Optional[int] = None
    customer_id: Optional[str] = None
    customer_user_id: Optional[str] = None
    owner_user_id: int
    responsible_user_id: Optional[int] = None
    article_count: int = 0
    create_time: datetime
    change_time: datetime


class TicketCreateRequest(RequestModel):
    """Body of POST /api/v1/tickets; ``body`` becomes the first article."""

    title: str
    queue_id: int
    body: Optional[str] = None
    priority_id: Optional[int] = None
    state_id: Optional[int] = None
    type_id: Optional[int] = None
    customer_email: Optional[str] = None
    customer_id: Optional[str] = None
    customer_user_id: Optional[str] = None


class CreatedTicket(BaseGoatflowModel):
    """Data of POST /api/v1/tickets."""

    id: int
    tn: str
    title: str
    queue_id: int
    ticket_state_id: int
    ticket_priority_id: int


class TicketUpdateRequest(RequestModel):
    """Body of PUT /api/v1/tickets/{id}; unset fields are unchanged."""

    title: Optional[str] = None
    queue_id: Optional[int] = None
    type_id: Optional[int] = None
    state_id: Optional[int] = None
    priority_id: Optional[int] = None
    customer_user_id: Optional[str] = None
    customer_id: Optional[str] = None
    user_id: Optional[int] = None  # owner
    responsible_user_id: Optional[int] = None
    ticket_lock_id: Optional[int] = None


class TicketRecord(BaseGoatflowModel):
    """Data of PUT /api/v1/tickets/{id}: the ticket row after the update."""

    id: int
    tn: str
    title: str
    queue_id: int
    type_id: int
    state_id: int
    priority_id: int
    user_id: int
    responsible_user_id: Optional[int] = None
    ticket_lock_id: int
    customer_user_id: str
    customer_id: str
    create_time: datetime
    create_by: int
    change_time: datetime
    change_by: int


class ReopenResult(BaseGoatflowModel):
    """Response of POST /api/v1/tickets/{id}/reopen."""

    id: int
    state_id: int
    state: str
    reason: str
    reopened_at: datetime


# Articles


class ArticleAttachment(BaseGoatflowModel):
    id: int
    filename: str
    content_type: str
    size: int
    disposition: str


class Article(BaseGoatflowModel):
    id: int
    ticket_id: int
    article_sender_type_id: int
    sender_type: Optional[str] = None
    communication_channel_id: int
    is_visible_for_customer: bool
    article_type: str  # e.g. email-external, note-internal, phone
    from_: Optional[str] = Field(default=None, alias="from")
    to: Optional[str] = None
    cc: Optional[str] = None
    subject: str
    body: str
    content_type: str
    message_id: Optional[str] = None
    create_time: Optional[datetime] = None
    create_by: int
    change_time: Optional[datetime] = None
    change_by: Optional[int] = None
    attachments: Optional[List[ArticleAttachment]] = None  # with include_attachments


class ArticleList(BaseGoatflowModel):
    """GET /api/v1/tickets/{id}/articles, newest first."""

    articles: List[Article]
    total: int


class ArticleCreateRequest(RequestModel):
    """Body of POST /api/v1/tickets/{id}/articles; ``body`` is required."""

    body: str
    subject: Optional[str] = None
    content_type: Optional[str] = None
    article_type: Optional[str] = None  # note-internal (note), note-external, email-external (email), phone
    sender_type: Optional[Literal["agent", "customer", "system"]] = None
    is_visible_for_customer: Optional[bool] = None
    from_: Optional[str] = Field(default=None, alias="from")
    to: Optional[str] = None
    cc: Optional[str] = None
    time_unit: Optional[float] = None


class ArticleUpdateRequest(RequestModel):
    """Body of PUT /api/v1/tickets/{id}/articles/{aid}; at least one field."""

    subject: Optional[str] = None
    body: Optional[str] = None


class ArticleUpdate(BaseGoatflowModel):
    id: int
    ticket_id: int
    subject: str
    body: str


# Users


class UserGroup(BaseGoatflowModel):
    """Group membership with permission keys (ro, move_into, create, note, owner, priority, rw)."""

    id: int
    name: str
    permissions: List[str] = []


class User(BaseGoatflowModel):
    """An agent from GET /api/v1/users or /api/v1/users/{id}."""

    id: int
    login: str
    first_name: Optional[str] = None
    last_name: Optional[str] = None
    valid_id: int
    valid: bool
    create_time: Optional[datetime] = None
    change_time: Optional[datetime] = None
    groups: List[UserGroup] = []
    email: Optional[str] = None  # only GET /api/v1/users/{id}
    preferences: Optional[Dict[str, str]] = None  # only GET /api/v1/users/{id}


class UserList(BaseGoatflowModel):
    users: List[User]
    pagination: Pagination


class CurrentUser(BaseGoatflowModel):
    """GET /api/v1/users/me."""

    id: int
    login: str
    email: str
    first_name: str
    last_name: str
    active: bool
    groups: List[GroupRef] = []


class UserCreateRequest(RequestModel):
    """Body of POST /api/v1/users; password needs 8+ characters."""

    login: str
    email: str
    password: str
    first_name: Optional[str] = None
    last_name: Optional[str] = None
    valid_id: Optional[int] = None
    groups: Optional[List[int]] = None  # group IDs


class CreatedUser(BaseGoatflowModel):
    """Data of POST /api/v1/users."""

    id: int
    login: str
    email: str
    first_name: Optional[str] = None
    last_name: Optional[str] = None
    valid_id: int
    valid: bool
    groups: List[int] = []
    created_at: datetime


class UserUpdateRequest(RequestModel):
    """Body of PUT /api/v1/users/{id}; unset fields are unchanged."""

    email: Optional[str] = None
    first_name: Optional[str] = None
    last_name: Optional[str] = None
    password: Optional[str] = None
    valid_id: Optional[int] = None


# Queues


class Queue(BaseGoatflowModel):
    """A ticket queue.

    ``valid``, ``comment``, ``create_time``, ``change_time`` and the ticket
    counts are only sent by GET /api/v1/queues; ``comments``,
    ``salutation_id`` and ``signature_id`` only by GET /api/v1/queues/{id}.
    """

    id: int
    name: str
    valid_id: int
    valid: Optional[bool] = None
    group_id: Optional[int] = None
    group_name: Optional[str] = None
    groups: List[GroupRef] = []
    system_address_id: Optional[int] = None
    salutation_id: Optional[int] = None
    signature_id: Optional[int] = None
    unlock_timeout: Optional[int] = None
    follow_up_id: Optional[int] = None
    follow_up_lock: Optional[int] = None
    comment: Optional[str] = None
    comments: Optional[str] = None
    create_time: Optional[datetime] = None
    change_time: Optional[datetime] = None
    ticket_count: Optional[int] = None
    open_tickets: Optional[int] = None
    closed_tickets: Optional[int] = None
    pending_tickets: Optional[int] = None


# Statistics


class StatisticsOverview(BaseGoatflowModel):
    total_tickets: int
    open_tickets: int
    closed_tickets: int
    pending_tickets: int


class QueueCount(BaseGoatflowModel):
    queue_id: int
    queue_name: str
    count: int


class PriorityCount(BaseGoatflowModel):
    priority_id: int
    priority_name: str
    count: int


class RecentActivity(BaseGoatflowModel):
    type: str
    ticket_id: int
    ticket_tn: str
    timestamp: datetime


class DashboardStatistics(BaseGoatflowModel):
    """GET /api/v1/statistics/dashboard, limited to the caller's readable queues."""

    overview: StatisticsOverview
    by_queue: List[QueueCount]
    by_priority: List[PriorityCount]
    recent_activity: List[RecentActivity]  # the ten newest tickets


# Search


class SearchQuery(RequestModel):
    """Body of POST /api/v1/search."""

    query: str
    types: Optional[List[str]] = None  # default ticket, article, customer
    filters: Optional[Dict[str, str]] = None
    offset: Optional[int] = None
    limit: Optional[int] = None  # default 20, max 100
    sort_by: Optional[str] = None
    sort_order: Optional[Literal["asc", "desc"]] = None
    highlight: Optional[bool] = None
    facets: Optional[List[str]] = None


class SearchHit(BaseGoatflowModel):
    id: str
    type: str
    score: float
    title: str
    content: str
    highlights: Optional[Dict[str, List[str]]] = None
    metadata: Dict[str, Any] = {}


class SearchFacet(BaseGoatflowModel):
    value: str
    count: int


class SearchResults(BaseGoatflowModel):
    """Response of POST /api/v1/search; ``warning`` is set when the backend is unavailable."""

    query: Optional[str] = None
    total_hits: int
    took_ms: int
    hits: List[SearchHit]
    facets: Optional[Dict[str, List[SearchFacet]]] = None
    suggestions: Optional[List[str]] = None
    warning: Optional[str] = None


# Webhooks


class Webhook(BaseGoatflowModel):
    """Represents an outbound webhook. The secret is write-only."""

    id: int
    name: str
    url: str
    events: List[str]
    headers: Dict[str, str] = {}
    has_secret: bool = False
    secret_hint: Optional[str] = None
    retry_count: int
    timeout_seconds: int
    is_active: bool
    created_at: datetime
    created_by: int
    updated_at: datetime
    updated_by: int


class WebhookRequest(RequestModel):
    """Body of POST and PUT /api/v1/webhooks; on update, unset fields are unchanged."""

    name: Optional[str] = None
    url: Optional[str] = None
    events: Optional[List[str]] = None
    secret: Optional[str] = None  # 16-512 characters; "" removes the secret
    headers: Optional[Dict[str, str]] = None
    retry_count: Optional[int] = None
    timeout_seconds: Optional[int] = None
    is_active: Optional[bool] = None


class WebhookDelivery(BaseGoatflowModel):
    """Represents one event sent (or scheduled) to one webhook."""

    id: int
    webhook_id: int
    event: str
    status: str  # pending, delivering, delivered, failed
    success: bool
    attempts: int
    status_code: Optional[int] = None
    error: Optional[str] = None
    duration_ms: Optional[int] = None
    next_attempt_at: Optional[datetime] = None
    delivered_at: Optional[datetime] = None
    payload: Optional[str] = None
    response: Optional[str] = None
    created_at: datetime
    updated_at: datetime


# Auth


class LoginUser(BaseGoatflowModel):
    id: int
    login: str
    email: str
    first_name: str
    last_name: str
    role: str


class TokenPair(BaseGoatflowModel):
    """Response of POST /api/v1/auth/login and POST /api/v1/auth/refresh.

    Every refresh rotates the refresh token.
    """

    user: LoginUser
    access_token: str
    refresh_token: str
    token_type: str
    expires_in: int  # access token lifetime, seconds
    refresh_expires_in: int  # refresh token lifetime, seconds


class Health(BaseGoatflowModel):
    """GET /health."""

    status: str
    components: Dict[str, str] = {}
    version: str
