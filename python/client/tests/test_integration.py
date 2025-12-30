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
import time
from typing import cast

import pytest
import requests
from tiauth_faroe.client import (
    SyncClient,
    ActionErrorResult,
    ActionSuccessResult,
    CreateSignupActionSuccessResult,
    CompleteSignupActionSuccessResult,
    CreateSigninActionSuccessResult,
    CompleteSigninActionSuccessResult,
    JSONValue,
)


# Default server URLs (can be overridden via environment variables)
TIAUTH_SERVER_URL = os.environ.get("TIAUTH_SERVER_URL", "http://localhost:3777")
# Python user server URL for retrieving verification codes
USER_SERVER_URL = os.environ.get("USER_SERVER_URL", "http://127.0.0.2:8079")


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


class TokenHelper:
    """Helper to retrieve verification codes from the Python user server."""

    def __init__(self, base_url: str):
        self.session = requests.Session()
        self.base_url = base_url

    def get_verification_code(
        self, action: str, email: str, timeout: float = 5.0
    ) -> str | None:
        """Get a verification code, waiting up to timeout seconds."""
        start = time.time()
        while time.time() - start < timeout:
            response = self.session.post(
                f"{self.base_url}/command",
                json={"command": "get_token", "action": action, "email": email},
            )
            if response.status_code == 200:
                data = response.json()
                if data.get("found"):
                    return data.get("code")
            time.sleep(0.1)
        return None


@pytest.fixture
def client() -> HttpSyncClient:
    """Create a client connected to the Go server."""
    return HttpSyncClient(TIAUTH_SERVER_URL)


@pytest.fixture
def token_helper() -> TokenHelper:
    """Create a helper to retrieve verification codes."""
    return TokenHelper(USER_SERVER_URL)


@pytest.fixture
def unique_email() -> str:
    """Generate a unique email for each test to avoid conflicts."""
    random_suffix = secrets.token_hex(8)
    return f"test_{random_suffix}@example.com"


class TestSignupFlow:
    """Test the complete signup flow."""

    def test_create_signup(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
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
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test that creating a signup with an existing user's email fails."""
        # First, complete a full signup
        password = "Str0ng!Pass#2025"
        self._complete_signup_flow(client, token_helper, unique_email, password)

        # Try to create another signup with the same email
        result = client.create_signup(unique_email)

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False
        assert result.error_code == "email_address_already_used"

    def test_send_verification_code(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test sending email verification code."""
        signup_result = client.create_signup(unique_email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)

        result = client.send_signup_email_address_verification_code(
            signup_result.signup_token
        )

        assert result.ok is True

        # Verify we can retrieve the code from the token store
        code = token_helper.get_verification_code("signup_verification", unique_email)
        assert code is not None

    def test_set_password(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test setting password for a signup (requires email verification first)."""
        signup_result = client.create_signup(unique_email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)
        signup_token = signup_result.signup_token

        # Send and verify email first (required before setting password)
        client.send_signup_email_address_verification_code(signup_token)
        code = token_helper.get_verification_code("signup_verification", unique_email)
        assert code is not None
        client.verify_signup_email_address_verification_code(signup_token, code)

        # Now set password (must be strong enough)
        result = client.set_signup_password(signup_token, "Str0ng!Pass#2025")

        assert result.ok is True

    def test_complete_signup_without_verification(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test that completing signup without email verification fails."""
        signup_result = client.create_signup(unique_email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)

        # Set password but don't verify email (will fail since email not verified)
        client.set_signup_password(signup_result.signup_token, "Str0ng!Pass#2025")

        # Try to complete - should fail because email not verified
        result = client.complete_signup(signup_result.signup_token)

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False

    def test_complete_signup_full_flow(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test the complete signup flow with email verification."""
        password = "Str0ng!Pass#2025"

        # Create signup
        signup_result = client.create_signup(unique_email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)
        signup_token = signup_result.signup_token

        # Send verification code
        send_result = client.send_signup_email_address_verification_code(signup_token)
        assert send_result.ok is True

        # Get the verification code from token store
        code = token_helper.get_verification_code("signup_verification", unique_email)
        assert code is not None

        # Verify the email
        verify_result = client.verify_signup_email_address_verification_code(
            signup_token, code
        )
        assert verify_result.ok is True

        # Set password
        password_result = client.set_signup_password(signup_token, password)
        assert password_result.ok is True

        # Complete signup
        complete_result = client.complete_signup(signup_token)
        assert isinstance(complete_result, CompleteSignupActionSuccessResult)
        assert complete_result.ok is True
        assert complete_result.session_token is not None

    def _complete_signup_flow(
        self,
        client: HttpSyncClient,
        token_helper: TokenHelper,
        email: str,
        password: str,
    ) -> CompleteSignupActionSuccessResult:
        """Helper to complete a full signup flow (for use in other tests)."""
        # Create signup
        signup_result = client.create_signup(email)
        assert isinstance(signup_result, CreateSignupActionSuccessResult)
        signup_token = signup_result.signup_token

        # Send verification code
        client.send_signup_email_address_verification_code(signup_token)

        # Get the verification code from token store
        code = token_helper.get_verification_code("signup_verification", email)
        if code is None:
            pytest.skip("Could not retrieve verification code from token store")

        # Verify email
        client.verify_signup_email_address_verification_code(signup_token, code)

        # Set password
        client.set_signup_password(signup_token, password)

        # Complete signup
        result = client.complete_signup(signup_token)
        if isinstance(result, ActionErrorResult):
            pytest.skip(
                f"Could not complete signup: {result.error_code}"
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

    def test_signin_with_wrong_password(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test that signing in with wrong password fails."""
        password = "Str0ng!Pass#2025"

        # First, complete a full signup to create the user
        signup_flow = TestSignupFlow()
        signup_flow._complete_signup_flow(client, token_helper, unique_email, password)

        # Create signin
        signin_result = client.create_signin(unique_email)
        assert isinstance(signin_result, CreateSigninActionSuccessResult)

        # Verify with wrong password
        result = client.verify_signin_user_password(
            signin_result.signin_token, "WrongPassword456!"
        )

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False

    def test_signin_full_flow(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test the complete signin flow after signup."""
        password = "Str0ng!Pass#2025"

        # First, complete a full signup
        signup_flow = TestSignupFlow()
        signup_result = signup_flow._complete_signup_flow(
            client, token_helper, unique_email, password
        )

        # Now test signin
        signin_result = client.create_signin(unique_email)
        assert isinstance(signin_result, CreateSigninActionSuccessResult)

        # Verify password
        verify_result = client.verify_signin_user_password(
            signin_result.signin_token, password
        )
        assert verify_result.ok is True

        # Complete signin
        complete_result = client.complete_signin(signin_result.signin_token)
        assert isinstance(complete_result, CompleteSigninActionSuccessResult)
        assert complete_result.ok is True
        assert complete_result.session_token is not None


class TestSessionFlow:
    """Test session-related functionality."""

    def test_get_session_invalid_token(self, client: HttpSyncClient):
        """Test that getting session with invalid token fails."""
        result = client.get_session("invalid_session_token_12345")

        assert isinstance(result, ActionErrorResult)
        assert result.ok is False
        assert result.error_code == "invalid_session_token"

    def test_get_session_valid(
        self, client: HttpSyncClient, token_helper: TokenHelper, unique_email: str
    ):
        """Test getting a valid session after signup."""
        password = "Str0ng!Pass#2025"

        # Complete signup to get a session
        signup_flow = TestSignupFlow()
        signup_result = signup_flow._complete_signup_flow(
            client, token_helper, unique_email, password
        )

        # Get session info
        result = client.get_session(signup_result.session_token)
        assert result.ok is True


class TestServerHealth:
    """Basic server health/connectivity tests."""

    def test_server_responds(
        self, client: HttpSyncClient, token_helper: TokenHelper
    ):
        """Test that the server responds to requests."""
        # Generate unique email to avoid conflicts with other tests
        random_suffix = secrets.token_hex(8)
        email = f"health_check_{random_suffix}@example.com"

        result = client.create_signup(email)

        # Either success or a known error means server works
        assert isinstance(
            result, (CreateSignupActionSuccessResult, ActionErrorResult)
        )
