"""HTTP client for the GoatFlow SDK."""

import asyncio
from typing import Any, Dict, Optional, Union, Type, TypeVar, List
from urllib.parse import urljoin, urlencode

import httpx
from pydantic import BaseModel

from .auth import Authenticator, NoAuth
from .exceptions import (
    GoatflowError,
    NetworkError,
    TimeoutError,
    NotFoundError,
    UnauthorizedError,
    ForbiddenError,
    RateLimitError,
    ServerError,
)
from .models import APIResponse

T = TypeVar("T", bound=BaseModel)


class HTTPClient:
    """HTTP client for making requests to the GoatFlow API."""
    
    def __init__(
        self,
        base_url: str,
        auth: Optional[Authenticator] = None,
        timeout: float = 30.0,
        retries: int = 3,
        user_agent: str = "goatflow-python-sdk/1.0.0",
        debug: bool = False,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.auth = auth or NoAuth()
        self.timeout = timeout
        self.retries = retries
        self.user_agent = user_agent
        self.debug = debug
        
        # Create HTTP client
        self._client = httpx.AsyncClient(
            timeout=httpx.Timeout(timeout),
            headers={"User-Agent": user_agent},
            follow_redirects=True,
        )
    
    async def __aenter__(self) -> "HTTPClient":
        """Async context manager entry."""
        return self
    
    async def __aexit__(self, exc_type: Any, exc_val: Any, exc_tb: Any) -> None:
        """Async context manager exit."""
        await self.close()
    
    async def close(self) -> None:
        """Close the HTTP client."""
        await self._client.aclose()
    
    def set_auth(self, auth: Authenticator) -> None:
        """Set the authentication method."""
        self.auth = auth
    
    async def _prepare_headers(self, headers: Optional[Dict[str, str]] = None) -> Dict[str, str]:
        """Prepare headers with authentication."""
        final_headers = {"Content-Type": "application/json"}
        
        if headers:
            final_headers.update(headers)
        
        # Add authentication headers
        if self.auth.is_expired():
            await self.auth.refresh()
        
        auth_headers = self.auth.get_auth_headers()
        final_headers.update(auth_headers)
        
        return final_headers
    
    def _build_url(self, path: str, params: Optional[Dict[str, Any]] = None) -> str:
        """Build the full URL for a request."""
        url = urljoin(self.base_url + "/", path.lstrip("/"))
        
        if params:
            # Filter out None values and convert to strings
            clean_params = {}
            for key, value in params.items():
                if value is not None:
                    if isinstance(value, list):
                        clean_params[key] = ",".join(str(v) for v in value)
                    else:
                        clean_params[key] = str(value)
            
            if clean_params:
                url += "?" + urlencode(clean_params)
        
        return url
    
    def _handle_error(self, response: httpx.Response) -> None:
        """Handle HTTP error responses."""
        status_code = response.status_code
        
        try:
            error_data = response.json()
        except Exception:
            error_data = {"error": response.text or "Unknown error"}
        
        message = error_data.get("message", error_data.get("error", "Unknown error"))
        code = error_data.get("code", "")
        details = error_data.get("details", "")
        
        if status_code == 404:
            raise NotFoundError(message, details=details, response_data=error_data)
        elif status_code == 401:
            raise UnauthorizedError(message, details=details, response_data=error_data)
        elif status_code == 403:
            raise ForbiddenError(message, details=details, response_data=error_data)
        elif status_code == 429:
            retry_after = None
            if "Retry-After" in response.headers:
                try:
                    retry_after = int(response.headers["Retry-After"])
                except ValueError:
                    pass
            raise RateLimitError(
                message, retry_after=retry_after, details=details, response_data=error_data
            )
        elif 500 <= status_code < 600:
            raise ServerError(
                message, status_code=status_code, code=code, details=details, response_data=error_data
            )
        else:
            raise GoatflowError(
                message, status_code=status_code, code=code, details=details, response_data=error_data
            )
    
    async def _make_request_with_retries(
        self,
        method: str,
        url: str,
        headers: Dict[str, str],
        **kwargs: Any,
    ) -> httpx.Response:
        """Make a request with retry logic."""
        last_exception = None
        
        for attempt in range(self.retries + 1):
            try:
                response = await self._client.request(
                    method, url, headers=headers, **kwargs
                )
                
                if response.is_success:
                    return response
                
                # Don't retry client errors (4xx) except for rate limiting
                if 400 <= response.status_code < 500 and response.status_code != 429:
                    self._handle_error(response)
                
                # For server errors and rate limiting, retry with exponential backoff
                if attempt < self.retries:
                    delay = 2 ** attempt
                    if self.debug:
                        print(f"Request failed (attempt {attempt + 1}), retrying in {delay}s...")
                    await asyncio.sleep(delay)
                    continue
                
                # Last attempt, raise the error
                self._handle_error(response)
                
            except httpx.TimeoutException as e:
                last_exception = TimeoutError(f"Request timed out after {self.timeout}s")
                if attempt < self.retries:
                    delay = 2 ** attempt
                    if self.debug:
                        print(f"Request timed out (attempt {attempt + 1}), retrying in {delay}s...")
                    await asyncio.sleep(delay)
                    continue
            except httpx.NetworkError as e:
                last_exception = NetworkError(f"Network error: {e}")
                if attempt < self.retries:
                    delay = 2 ** attempt
                    if self.debug:
                        print(f"Network error (attempt {attempt + 1}), retrying in {delay}s...")
                    await asyncio.sleep(delay)
                    continue
        
        # If we get here, all retries failed
        if last_exception:
            raise last_exception
        
        raise GoatflowError("Request failed after all retry attempts")
    
    def _extract_data(self, response: httpx.Response, model_class: Optional[Type[T]] = None) -> Any:
        """Extract data from response."""
        try:
            data = response.json()
        except Exception:
            return response.text
        
        # Handle standard API response format
        if isinstance(data, dict) and "success" in data:
            if not data["success"]:
                error_msg = data.get("error", "API request failed")
                raise GoatflowError(error_msg, status_code=response.status_code)
            
            result_data = data.get("data", data)
        else:
            result_data = data
        
        # Parse with Pydantic model if provided
        if model_class and result_data is not None:
            if isinstance(result_data, list):
                return [model_class.model_validate(item) for item in result_data]
            else:
                return model_class.model_validate(result_data)
        
        return result_data
    
    async def get(
        self,
        path: str,
        params: Optional[Dict[str, Any]] = None,
        headers: Optional[Dict[str, str]] = None,
        model_class: Optional[Type[T]] = None,
    ) -> Any:
        """Make a GET request."""
        url = self._build_url(path, params)
        request_headers = await self._prepare_headers(headers)
        
        response = await self._make_request_with_retries("GET", url, request_headers)
        return self._extract_data(response, model_class)
    
    async def post(
        self,
        path: str,
        data: Optional[Any] = None,
        json: Optional[Any] = None,
        headers: Optional[Dict[str, str]] = None,
        model_class: Optional[Type[T]] = None,
    ) -> Any:
        """Make a POST request."""
        url = self._build_url(path)
        request_headers = await self._prepare_headers(headers)
        
        kwargs = {}
        if json is not None:
            if isinstance(json, BaseModel):
                kwargs["json"] = json.model_dump(exclude_none=True)
            else:
                kwargs["json"] = json
        elif data is not None:
            kwargs["data"] = data
        
        response = await self._make_request_with_retries("POST", url, request_headers, **kwargs)
        return self._extract_data(response, model_class)
    
    async def put(
        self,
        path: str,
        data: Optional[Any] = None,
        json: Optional[Any] = None,
        headers: Optional[Dict[str, str]] = None,
        model_class: Optional[Type[T]] = None,
    ) -> Any:
        """Make a PUT request."""
        url = self._build_url(path)
        request_headers = await self._prepare_headers(headers)
        
        kwargs = {}
        if json is not None:
            if isinstance(json, BaseModel):
                kwargs["json"] = json.model_dump(exclude_none=True)
            else:
                kwargs["json"] = json
        elif data is not None:
            kwargs["data"] = data
        
        response = await self._make_request_with_retries("PUT", url, request_headers, **kwargs)
        return self._extract_data(response, model_class)
    
    async def delete(
        self,
        path: str,
        headers: Optional[Dict[str, str]] = None,
        model_class: Optional[Type[T]] = None,
    ) -> Any:
        """Make a DELETE request."""
        url = self._build_url(path)
        request_headers = await self._prepare_headers(headers)
        
        response = await self._make_request_with_retries("DELETE", url, request_headers)
        return self._extract_data(response, model_class)
    
    async def patch(
        self,
        path: str,
        data: Optional[Any] = None,
        json: Optional[Any] = None,
        headers: Optional[Dict[str, str]] = None,
        model_class: Optional[Type[T]] = None,
    ) -> Any:
        """Make a PATCH request."""
        url = self._build_url(path)
        request_headers = await self._prepare_headers(headers)
        
        kwargs = {}
        if json is not None:
            if isinstance(json, BaseModel):
                kwargs["json"] = json.model_dump(exclude_none=True)
            else:
                kwargs["json"] = json
        elif data is not None:
            kwargs["data"] = data
        
        response = await self._make_request_with_retries("PATCH", url, request_headers, **kwargs)
        return self._extract_data(response, model_class)
    
    async def upload_file(
        self,
        path: str,
        file_data: bytes,
        filename: str,
        content_type: Optional[str] = None,
        additional_fields: Optional[Dict[str, str]] = None,
        headers: Optional[Dict[str, str]] = None,
        model_class: Optional[Type[T]] = None,
    ) -> Any:
        """Upload a file using multipart/form-data."""
        url = self._build_url(path)
        request_headers = await self._prepare_headers(headers)
        
        # Remove Content-Type header for multipart uploads
        if "Content-Type" in request_headers:
            del request_headers["Content-Type"]
        
        files = {"file": (filename, file_data, content_type)}
        data = additional_fields or {}
        
        response = await self._make_request_with_retries(
            "POST", url, request_headers, files=files, data=data
        )
        return self._extract_data(response, model_class)
    
    async def download_file(self, path: str, headers: Optional[Dict[str, str]] = None) -> bytes:
        """Download a file and return the raw bytes."""
        url = self._build_url(path)
        request_headers = await self._prepare_headers(headers)
        
        response = await self._make_request_with_retries("GET", url, request_headers)
        
        if not response.is_success:
            self._handle_error(response)
        
        return response.content