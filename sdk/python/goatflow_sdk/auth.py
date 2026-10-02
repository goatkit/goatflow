"""Credentials for the GoatFlow SDK.

GoatFlow accepts two kinds, both sent as ``Authorization: Bearer <token>``:
API tokens (``gf_...``) and JWT access tokens from ``POST /api/v1/auth/login``.
"""

import asyncio
from abc import ABC, abstractmethod
from datetime import datetime, timedelta, timezone
from typing import Awaitable, Callable, Dict, Optional, Tuple

from .exceptions import AuthenticationError

#: Called with the refresh token when the access token is about to expire;
#: returns ``(access_token, refresh_token, expires_at)``. GoatFlow has no
#: refresh endpoint, so a typical implementation logs in again.
RefreshFunction = Callable[
    [Optional[str]], Awaitable[Tuple[str, Optional[str], Optional[datetime]]]
]


class Authenticator(ABC):
    """Supplies the headers that authenticate a request."""

    @abstractmethod
    async def headers(self) -> Dict[str, str]:
        """Headers to add to a request, refreshing credentials first if needed."""


class NoAuth(Authenticator):
    """No credentials (only login and /health work without them)."""

    async def headers(self) -> Dict[str, str]:
        return {}


class APIKeyAuth(Authenticator):
    """A GoatFlow API token (``gf_...``)."""

    def __init__(self, api_key: str) -> None:
        self.api_key = api_key

    async def headers(self) -> Dict[str, str]:
        return {"Authorization": f"Bearer {self.api_key}"}


class JWTAuth(Authenticator):
    """A JWT access token.

    When ``expires_at`` is less than a minute ahead, ``refresh_function`` is
    awaited before the request; without one an expired token raises
    :class:`AuthenticationError`.
    """

    def __init__(
        self,
        token: str,
        refresh_token: Optional[str] = None,
        expires_at: Optional[datetime] = None,
        refresh_function: Optional[RefreshFunction] = None,
    ) -> None:
        self.token = token
        self.refresh_token = refresh_token
        self.expires_at = expires_at
        self.refresh_function = refresh_function
        self._lock = asyncio.Lock()

    def _expired(self) -> bool:
        if self.expires_at is None:
            return False
        expires_at = self.expires_at
        if expires_at.tzinfo is None:
            expires_at = expires_at.replace(tzinfo=timezone.utc)
        return expires_at - timedelta(minutes=1) <= datetime.now(timezone.utc)

    async def headers(self) -> Dict[str, str]:
        if self._expired():
            async with self._lock:
                if self._expired():
                    if self.refresh_function is None:
                        raise AuthenticationError(
                            f"Access token expired at {self.expires_at} and no refresh_function is configured"
                        )
                    token, refresh_token, expires_at = await self.refresh_function(self.refresh_token)
                    self.token = token
                    self.refresh_token = refresh_token or self.refresh_token
                    self.expires_at = expires_at
        return {"Authorization": f"Bearer {self.token}"}
