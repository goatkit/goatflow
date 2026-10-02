"""Exceptions raised by the GoatFlow SDK."""

from typing import Optional


class GoatflowError(Exception):
    """A non-successful API response.

    Raised for an HTTP status outside 2xx, or a 2xx response whose envelope
    says ``{"success": false}``. ``code`` is set when the API sent one (e.g.
    ``core:invalid_token``); ``body`` holds the raw body when it was not JSON.
    """

    def __init__(
        self,
        message: str,
        status_code: Optional[int] = None,
        code: Optional[str] = None,
        body: Optional[str] = None,
    ) -> None:
        super().__init__(message)
        self.message = message
        self.status_code = status_code
        self.code = code
        self.body = body

    def __str__(self) -> str:
        text = self.message
        if self.status_code is not None:
            text = f"HTTP {self.status_code}: {text}"
        if self.code:
            text = f"{text} ({self.code})"
        return text


class AuthenticationError(GoatflowError):
    """The client could not produce credentials (e.g. an expired token)."""


class UnauthorizedError(AuthenticationError):
    """HTTP 401."""


class ForbiddenError(AuthenticationError):
    """HTTP 403."""


class NotFoundError(GoatflowError):
    """HTTP 404."""


class RateLimitError(GoatflowError):
    """HTTP 429. ``retry_after`` is the Retry-After header in seconds, if sent."""

    def __init__(
        self,
        message: str,
        status_code: Optional[int] = None,
        code: Optional[str] = None,
        body: Optional[str] = None,
        retry_after: Optional[int] = None,
    ) -> None:
        super().__init__(message, status_code, code, body)
        self.retry_after = retry_after


class ServerError(GoatflowError):
    """HTTP 5xx."""


class NetworkError(GoatflowError):
    """The request produced no HTTP response."""


class TimeoutError(NetworkError):
    """The request timed out."""


class ResponseShapeError(GoatflowError):
    """A successful response whose body did not match the expected model."""
