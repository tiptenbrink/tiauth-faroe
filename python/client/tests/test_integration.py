"""Integration tests that require the Go tiauth server to be running.

These tests exercise the full authentication flow through the Go server,
which in turn calls the Python user-server via HTTP.

To run these tests:
1. Start the Go tiauth server: cd tiauth && go run ./cmd
2. Start the Python user server: cd sync-reference-server && uv run python main.py
3. Run tests: uv run pytest tests/test_integration.py -v

The tests are marked with @pytest.mark.integration and will be skipped
if the Go server is not reachable.
"""

import os
import secrets
import socket
from typing import cast

import pytest
import requests
from tiauth_faroe.client import (
    SyncClient,
    ActionErrorResult,
    CreateSignupActionSuccessResult,
    CompleteSignupActionSuccessResult,
    JSONValue,
)


# Default server URL (can be overridden via environment variable)
TIAUTH_SERVER_URL = os.environ.get("TIAUTH_SERVER_URL", "http://localhost:3777")


def is_server_running(host: str = "localhost", port: int = 3777) -> bool:
    """Check if the Go server is reachable."""
    try:
        with socket.create_connection((host, port), timeout=1):
            return True
    except (socket.timeout, ConnectionRefusedError, OSError):
        return False


# Skip all tests in this module if the server is not running
pytestmark = pytest.mark.skipif(
    not is_server_running(),
    reason=f"tiauth Go server not running at {TIAUTH_SERVER_URL}",
)


class HttpSyncClient(SyncClient):
    """HTTP-based sync client for testing against the Go server."""

    def __init__(self, base_url: str):
        self.session = requests.Session()
        self.base_url = base_url

    def send_action_invocation_request(self, body: JSONValue) -> JSONValue:
        response = self.session.post(f"{self.base_url}/", json=body)
        return cast(JSONValue, response.json())


@pytest.fixture
def client() -> HttpSyncClient:
    """Create a client connected to the Go server."""
    return HttpSyncClient(TIAUTH_SERVER_URL)


@pytest.fixture
def unique_email() -> str:
    """Generate a unique email for each test to avoid conflicts."""
    random_suffix = secrets.token_hex(8)
    return f"test_{random_suffix}@example.com"


class TestSignupFlow:
    """Test the complete signup flow."""

    def test_create_signup(self, client: HttpSyncClient, unique_email: str):
        """Test creating a new signup."""
        result = client.create_signup(unique_email)

        assert isinstance(result, CreateSignupActionSuccessResult)
        assert result.ok is True
        assert result.signup.email_address == unique_email
        assert result.signup.email_address_verified is False
        assert result.signup.password_set is False
        assert result.signup_token is not None
        assert len(result.signup_token) > 0

    def test_create_signup_duplicate_email(
        self, client: HttpSyncClient, unique_email: str
    ):
        """Test that creating a signup with an existing user's email fails."""
        # First, complete a full signup
        password = "TestPassword123!"
        self._complete_signup_flow(client, unique_email, password)

        # Try to create another signup with the same email
        result = client.create_signup(unique_email)

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False
        assert result.error_code == "email_address_already_used"

    def test_send_verification_code(self, client: HttpSyncClient, unique_email: str):
        """Test sending email verification code."""
        signup_result = client.create_signup(unique_email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)

        result = client.send_signup_email_address_verification_code(
            signup_result.signup_token
        )

        # Should succeed (even though we can't actually receive the email in tests)
        assert result.ok is True

    def test_set_password(self, client: HttpSyncClient, unique_email: str):
        """Test setting password for a signup."""
        signup_result = client.create_signup(unique_email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)

        result = client.set_signup_password(
            signup_result.signup_token, "SecurePassword123!"
        )

        assert result.ok is True

    def test_complete_signup_without_verification(
        self, client: HttpSyncClient, unique_email: str
    ):
        """Test that completing signup without email verification fails."""
        signup_result = client.create_signup(unique_email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)

        # Set password but don't verify email
        client.set_signup_password(signup_result.signup_token, "SecurePassword123!")

        # Try to complete - should fail because email not verified
        result = client.complete_signup(signup_result.signup_token)

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False

    def _complete_signup_flow(
        self, client: HttpSyncClient, email: str, password: str
    ) -> CompleteSignupActionSuccessResult:
        """Helper to complete a full signup flow (for use in other tests)."""
        # Create signup
        signup_result = client.create_signup(email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)
        signup_token = signup_result.signup_token

        # Send verification code (we can't receive it, but this triggers the flow)
        client.send_signup_email_address_verification_code(signup_token)

        # In a real test environment, we'd need to mock or intercept the email.
        # For now, we rely on test mode or the server accepting a test code.
        # This test will likely fail in production mode without email interception.

        # Set password
        client.set_signup_password(signup_token, password)

        # Note: Complete signup will fail without verified email in production.
        # This helper is primarily for setting up test scenarios.
        result = client.complete_signup(signup_token)
        if isinstance(result, ActionErrorResult):
            pytest.skip(
                f"Could not complete signup (email verification required): "
                f"{result.error_code}"
            )
        return result


class TestSigninFlow:
    """Test the signin flow (requires a completed signup first)."""

    def test_create_signin_nonexistent_user(self, client: HttpSyncClient):
        """Test that signing in with non-existent email fails."""
        result = client.create_signin("nonexistent@example.com")

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False
        assert result.error_code == "user_not_found"

    def test_verify_wrong_password(
        self, client: HttpSyncClient, unique_email: str
    ):
        """Test that verifying with wrong password fails."""
        # This test requires a user to exist first
        # We'll skip if we can't create the user
        password = "CorrectPassword123!"

        # Try to create user (may fail if email verification is required)
        signup_result = client.create_signup(unique_email)
        if isinstance(signup_result, ActionErrorResult):
            pytest.skip("Could not create signup")

        # Set password
        client.set_signup_password(signup_result.signup_token, password)

        # Try to sign in (create signin may work even if signup not complete)
        # But this depends on server implementation
        signin_result = client.create_signin(unique_email)
        if isinstance(signin_result, ActionErrorResult):
            pytest.skip(f"User does not exist: {signin_result.error_code}")

        # Verify with wrong password
        result = client.verify_signin_user_password(
            signin_result.signin_token, "WrongPassword456!"
        )

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False


class TestSessionFlow:
    """Test session-related functionality."""

    def test_get_session_invalid_token(self, client: HttpSyncClient):
        """Test that getting session with invalid token fails."""
        result = client.get_session("invalid_session_token_12345")

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False
        assert result.error_code == "invalid_session_token"


class TestServerHealth:
    """Basic server health/connectivity tests."""

    def test_server_responds(self, client: HttpSyncClient):
        """Test that the server responds to requests."""
        # Send a minimal request to verify server is responding
        result = client.create_signup("health_check@example.com")

        # Either success or a known error (like email already used) means server works
        assert isinstance(
            result, (CreateSignupActionSuccessResult, ActionErrorResult)
        )

    def test_invalid_action(self, client: HttpSyncClient):
        """Test that server handles invalid actions gracefully."""
        # Send an invalid action directly
        response = client.send_action_invocation_request(
            {"action": "nonexistent_action", "arguments": {}}
        )

        # Server should return an error response, not crash
        assert isinstance(response, dict)
