"""Authentication classes for the GoatFlow SDK."""

import asyncio
from abc import ABC, abstractmethod
from datetime import datetime, timezone
from typing import Optional, Dict, Any, Callable, Awaitable, Union

from .exceptions import AuthenticationError


class Authenticator(ABC):
    """Base class for authentication methods."""
    
    @abstractmethod
    def get_auth_headers(self) -> Dict[str, str]:
        """Get authentication headers for requests."""
        pass
    
    @abstractmethod
    def is_expired(self) -> bool:
        """Check if the authentication is expired."""
        pass
    
    @abstractmethod
    async def refresh(self) -> None:
        """Refresh the authentication if possible."""
        pass
    
    @property
    @abstractmethod
    def auth_type(self) -> str:
        """Get the authentication type."""
        pass


class APIKeyAuth(Authenticator):
    """API key authentication."""
    
    def __init__(self, api_key: str, header_name: str = "X-API-Key") -> None:
        self.api_key = api_key
        self.header_name = header_name
    
    def get_auth_headers(self) -> Dict[str, str]:
        """Get API key headers."""
        return {self.header_name: self.api_key}
    
    def is_expired(self) -> bool:
        """API keys don't expire."""
        return False
    
    async def refresh(self) -> None:
        """API keys don't need refreshing."""
        pass
    
    @property
    def auth_type(self) -> str:
        return "api-key"


class JWTAuth(Authenticator):
    """JWT token authentication with automatic refresh."""
    
    def __init__(
        self,
        token: str,
        refresh_token: Optional[str] = None,
        expires_at: Optional[datetime] = None,
        refresh_function: Optional[
            Callable[[str], Awaitable[Dict[str, Any]]]
        ] = None,
    ) -> None:
        self.token = token
        self.refresh_token = refresh_token
        self.expires_at = expires_at
        self.refresh_function = refresh_function
        self._refresh_lock = asyncio.Lock()
    
    def get_auth_headers(self) -> Dict[str, str]:
        """Get JWT authorization headers."""
        return {"Authorization": f"Bearer {self.token}"}
    
    def is_expired(self) -> bool:
        """Check if the JWT token is expired."""
        if not self.expires_at:
            return False
        
        # Add 1 minute buffer
        buffer_time = datetime.now(timezone.utc).timestamp() + 60
        return self.expires_at.timestamp() <= buffer_time
    
    async def refresh(self) -> None:
        """Refresh the JWT token."""
        if not self.refresh_function or not self.refresh_token:
            raise AuthenticationError("No refresh function or refresh token available")
        
        async with self._refresh_lock:
            # Check again in case another coroutine already refreshed
            if not self.is_expired():
                return
            
            try:
                result = await self.refresh_function(self.refresh_token)
                self.token = result["access_token"]
                self.refresh_token = result.get("refresh_token", self.refresh_token)
                
                if "expires_at" in result:
                    if isinstance(result["expires_at"], str):
                        self.expires_at = datetime.fromisoformat(
                            result["expires_at"].replace("Z", "+00:00")
                        )
                    else:
                        self.expires_at = result["expires_at"]
                        
            except Exception as e:
                raise AuthenticationError(f"Failed to refresh token: {e}") from e
    
    @property
    def auth_type(self) -> str:
        return "jwt"


class OAuth2Auth(Authenticator):
    """OAuth2 token authentication."""
    
    def __init__(
        self,
        access_token: str,
        refresh_token: Optional[str] = None,
        token_type: str = "Bearer",
        expires_at: Optional[datetime] = None,
        refresh_function: Optional[
            Callable[[str], Awaitable[Dict[str, Any]]]
        ] = None,
    ) -> None:
        self.access_token = access_token
        self.refresh_token = refresh_token
        self.token_type = token_type
        self.expires_at = expires_at
        self.refresh_function = refresh_function
        self._refresh_lock = asyncio.Lock()
    
    def get_auth_headers(self) -> Dict[str, str]:
        """Get OAuth2 authorization headers."""
        return {"Authorization": f"{self.token_type} {self.access_token}"}
    
    def is_expired(self) -> bool:
        """Check if the OAuth2 token is expired."""
        if not self.expires_at:
            return False
        
        # Add 1 minute buffer
        buffer_time = datetime.now(timezone.utc).timestamp() + 60
        return self.expires_at.timestamp() <= buffer_time
    
    async def refresh(self) -> None:
        """Refresh the OAuth2 token."""
        if not self.refresh_function or not self.refresh_token:
            raise AuthenticationError("No refresh function or refresh token available")
        
        async with self._refresh_lock:
            # Check again in case another coroutine already refreshed
            if not self.is_expired():
                return
            
            try:
                result = await self.refresh_function(self.refresh_token)
                self.access_token = result["access_token"]
                self.refresh_token = result.get("refresh_token", self.refresh_token)
                
                if "expires_at" in result:
                    if isinstance(result["expires_at"], str):
                        self.expires_at = datetime.fromisoformat(
                            result["expires_at"].replace("Z", "+00:00")
                        )
                    else:
                        self.expires_at = result["expires_at"]
                        
            except Exception as e:
                raise AuthenticationError(f"Failed to refresh token: {e}") from e
    
    @property
    def auth_type(self) -> str:
        return "oauth2"


class NoAuth(Authenticator):
    """No authentication."""
    
    def get_auth_headers(self) -> Dict[str, str]:
        """Return empty headers."""
        return {}
    
    def is_expired(self) -> bool:
        """Never expires."""
        return False
    
    async def refresh(self) -> None:
        """Nothing to refresh."""
        pass
    
    @property
    def auth_type(self) -> str:
        return "none"