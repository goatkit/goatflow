"""
GoatFlow Python SDK

Official Python SDK for the GoatFlow ticketing system API.

Basic usage:
    >>> from goatflow_sdk import GoatflowClient
    >>> client = GoatflowClient.with_api_key("https://your-goatflow.com", "your-api-key")
    >>> tickets = await client.tickets.list()
    >>> print(f"Found {tickets.total_count} tickets")

Authentication:
    # API Key
    client = GoatflowClient.with_api_key(base_url, api_key)
    
    # JWT Token
    client = GoatflowClient.with_jwt(base_url, token, refresh_token, expires_at)
    
    # OAuth2
    client = GoatflowClient.with_oauth2(base_url, access_token, refresh_token, expires_at)
    
    # Login flow
    client = GoatflowClient(base_url)
    await client.login("user@example.com", "password")
"""

from .client import GoatflowClient
from .exceptions import (
    GoatflowError,
    ValidationError,
    NetworkError,
    TimeoutError,
    NotFoundError,
    UnauthorizedError,
    ForbiddenError,
    RateLimitError,
)
from .models import (
    Ticket,
    TicketMessage,
    User,
    Queue,
    Attachment,
    Group,
    DashboardStats,
    SearchResult,
    InternalNote,
    NoteTemplate,
    LDAPUser,
    LDAPSyncResult,
    Webhook,
    WebhookDelivery,
    TicketCreateRequest,
    TicketUpdateRequest,
    TicketListOptions,
    MessageCreateRequest,
    UserCreateRequest,
    UserUpdateRequest,
    AuthLoginRequest,
    AuthLoginResponse,
)
from .auth import APIKeyAuth, JWTAuth, OAuth2Auth

__version__ = "1.0.0"
__author__ = "GoatFlow Team"
__email__ = "hello@goatflow.io"
__license__ = "MIT"

__all__ = [
    # Main client
    "GoatflowClient",
    # Exceptions
    "GoatflowError",
    "ValidationError",
    "NetworkError",
    "TimeoutError",
    "NotFoundError",
    "UnauthorizedError",
    "ForbiddenError", 
    "RateLimitError",
    # Models
    "Ticket",
    "TicketMessage",
    "User",
    "Queue",
    "Attachment",
    "Group",
    "DashboardStats",
    "SearchResult",
    "InternalNote",
    "NoteTemplate",
    "LDAPUser",
    "LDAPSyncResult",
    "Webhook",
    "WebhookDelivery",
    "TicketCreateRequest",
    "TicketUpdateRequest",
    "TicketListOptions",
    "MessageCreateRequest",
    "UserCreateRequest",
    "UserUpdateRequest",
    "AuthLoginRequest",
    "AuthLoginResponse",
    # Auth
    "APIKeyAuth",
    "JWTAuth",
    "OAuth2Auth",
]

# Convenience functions for error checking
def is_goatflow_error(error: Exception) -> bool:
    """Check if an exception is a GoatFlow API error."""
    return isinstance(error, GoatflowError)

def is_not_found_error(error: Exception) -> bool:
    """Check if an exception is a 404 Not Found error."""
    return isinstance(error, NotFoundError)

def is_unauthorized_error(error: Exception) -> bool:
    """Check if an exception is a 401 Unauthorized error."""
    return isinstance(error, UnauthorizedError)

def is_forbidden_error(error: Exception) -> bool:
    """Check if an exception is a 403 Forbidden error."""
    return isinstance(error, ForbiddenError)

def is_rate_limit_error(error: Exception) -> bool:
    """Check if an exception is a 429 Rate Limit error."""
    return isinstance(error, RateLimitError)

def is_validation_error(error: Exception) -> bool:
    """Check if an exception is a validation error."""
    return isinstance(error, ValidationError)

def is_network_error(error: Exception) -> bool:
    """Check if an exception is a network error."""
    return isinstance(error, NetworkError)

def is_timeout_error(error: Exception) -> bool:
    """Check if an exception is a timeout error."""
    return isinstance(error, TimeoutError)