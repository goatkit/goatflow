"""Exception classes for the GoatFlow SDK."""

from typing import Optional, Any


class GoatflowError(Exception):
    """Base exception for all GoatFlow API errors."""
    
    def __init__(
        self,
        message: str,
        status_code: Optional[int] = None,
        code: Optional[str] = None,
        details: Optional[str] = None,
        response_data: Optional[Any] = None,
    ) -> None:
        super().__init__(message)
        self.message = message
        self.status_code = status_code
        self.code = code
        self.details = details
        self.response_data = response_data
    
    def __str__(self) -> str:
        base_msg = self.message
        if self.status_code:
            base_msg = f"HTTP {self.status_code}: {base_msg}"
        if self.details:
            base_msg = f"{base_msg} - {self.details}"
        return base_msg
    
    def __repr__(self) -> str:
        return (
            f"{self.__class__.__name__}("
            f"message={self.message!r}, "
            f"status_code={self.status_code}, "
            f"code={self.code!r})"
        )


class ValidationError(GoatflowError):
    """Raised when request validation fails."""
    
    def __init__(
        self,
        message: str,
        field: Optional[str] = None,
        value: Optional[Any] = None,
        **kwargs: Any,
    ) -> None:
        super().__init__(message, **kwargs)
        self.field = field
        self.value = value


class NetworkError(GoatflowError):
    """Raised when a network operation fails."""
    
    def __init__(
        self,
        message: str,
        operation: Optional[str] = None,
        url: Optional[str] = None,
        **kwargs: Any,
    ) -> None:
        super().__init__(message, **kwargs)
        self.operation = operation
        self.url = url


class TimeoutError(GoatflowError):
    """Raised when a request times out."""
    
    def __init__(
        self,
        message: str,
        timeout: Optional[float] = None,
        **kwargs: Any,
    ) -> None:
        super().__init__(message, **kwargs)
        self.timeout = timeout


class AuthenticationError(GoatflowError):
    """Base class for authentication-related errors."""
    pass


class NotFoundError(GoatflowError):
    """Raised when a resource is not found (HTTP 404)."""
    
    def __init__(self, message: str = "Resource not found", **kwargs: Any) -> None:
        super().__init__(message, status_code=404, code="NOT_FOUND", **kwargs)


class UnauthorizedError(AuthenticationError):
    """Raised when authentication is required or invalid (HTTP 401)."""
    
    def __init__(self, message: str = "Unauthorized", **kwargs: Any) -> None:
        super().__init__(message, status_code=401, code="UNAUTHORIZED", **kwargs)


class ForbiddenError(AuthenticationError):
    """Raised when access is forbidden (HTTP 403)."""
    
    def __init__(self, message: str = "Forbidden", **kwargs: Any) -> None:
        super().__init__(message, status_code=403, code="FORBIDDEN", **kwargs)


class RateLimitError(GoatflowError):
    """Raised when rate limit is exceeded (HTTP 429)."""
    
    def __init__(
        self,
        message: str = "Rate limit exceeded",
        retry_after: Optional[int] = None,
        **kwargs: Any,
    ) -> None:
        super().__init__(message, status_code=429, code="RATE_LIMITED", **kwargs)
        self.retry_after = retry_after


class ServerError(GoatflowError):
    """Raised when the server returns a 5xx error."""
    
    def __init__(self, message: str = "Internal server error", **kwargs: Any) -> None:
        kwargs.setdefault("status_code", 500)
        kwargs.setdefault("code", "SERVER_ERROR")
        super().__init__(message, **kwargs)


class ConfigurationError(GoatflowError):
    """Raised when there's a configuration error."""
    
    def __init__(self, message: str, field: Optional[str] = None, **kwargs: Any) -> None:
        super().__init__(message, code="CONFIG_ERROR", **kwargs)
        self.field = field


class WebSocketError(GoatflowError):
    """Raised when WebSocket operations fail."""
    
    def __init__(self, message: str, **kwargs: Any) -> None:
        super().__init__(message, code="WEBSOCKET_ERROR", **kwargs)