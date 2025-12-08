"""
Example script demonstrating signup and signin flows using the Python client.

This script assumes:
- Faroe auth server is running on http://localhost:3777
- User backend is running on http://localhost:8000
- SMTP test server is running on http://localhost:3525
"""

import json
import urllib.request
import urllib.error
from tiauth_faroe.client import (
    SyncClient,
    ActionErrorResult,
    CompleteSignupActionSuccessResult,
    CompleteSigninActionSuccessResult,
    JSONValue,
)


class HTTPClient(SyncClient):
    """Simple HTTP client for testing."""

    def __init__(self, endpoint_url: str):
        self.endpoint_url = endpoint_url

    def send_action_invocation_request(self, body: JSONValue) -> JSONValue:
        """Send HTTP POST request to the action invocation endpoint."""
        body_json = json.dumps(body)
        req = urllib.request.Request(
            self.endpoint_url,
            data=body_json.encode("utf-8"),
            headers={"Content-Type": "application/json"},
        )

        try:
            with urllib.request.urlopen(req) as response:
                response_json = response.read().decode("utf-8")
                return json.loads(response_json)
        except urllib.error.HTTPError as e:
            error_body = e.read().decode("utf-8")
            raise Exception(f"HTTP {e.code}: {error_body}")


def fetch_verification_code(email: str) -> str:
    """Fetch the verification code from the SMTP test server."""
    url = f"http://localhost:3525/emails?email={urllib.parse.quote(email)}&category=signup"
    req = urllib.request.Request(url)

    with urllib.request.urlopen(req) as response:
        data = json.loads(response.read().decode("utf-8"))
        return data["data"]["code"]


def prepare_user(email: str, names: list[str]) -> None:
    """Prepare a user in the backend database."""
    url = "http://localhost:8000/private/prepare_user"
    data = json.dumps({"email": email, "names": names})
    req = urllib.request.Request(
        url,
        data=data.encode("utf-8"),
        headers={"Content-Type": "application/json"},
    )

    with urllib.request.urlopen(req) as response:
        response.read()


def test_signup_flow():
    """Test the complete signup flow."""
    print("=== Testing Signup Flow ===\n")

    client = HTTPClient("http://localhost:3777/")
    test_email = "test_python@example.com"
    test_password = "TestPassword123!"

    # Step 1: Prepare user
    print("1. Preparing user in database...")
    prepare_user(test_email, ["Test", "Python"])
    print(f"   ✓ User prepared: {test_email}\n")

    # Step 2: Create signup
    print("2. Creating signup...")
    signup_result = client.create_signup(test_email)
    if isinstance(signup_result, ActionErrorResult):
        print(f"   ✗ Signup failed: {signup_result.error_code}")
        return None
    print(f"   ✓ Signup created: {signup_result.signup_token}\n")

    # Step 3: Fetch verification code
    print("3. Fetching verification code from SMTP server...")
    verification_code = fetch_verification_code(test_email)
    print(f"   ✓ Verification code: {verification_code}\n")

    # Step 4: Verify email
    print("4. Verifying email address...")
    verify_result = client.verify_signup_email_address_verification_code(
        signup_result.signup_token, verification_code
    )
    if isinstance(verify_result, ActionErrorResult):
        print(f"   ✗ Verification failed: {verify_result.error_code}")
        return None
    print("   ✓ Email verified\n")

    # Step 5: Set password
    print("5. Setting password...")
    password_result = client.set_signup_password(
        signup_result.signup_token, test_password
    )
    if isinstance(password_result, ActionErrorResult):
        print(f"   ✗ Set password failed: {password_result.error_code}")
        return None
    print("   ✓ Password set\n")

    # Step 6: Complete signup
    print("6. Completing signup...")
    complete_result = client.complete_signup(signup_result.signup_token)
    if isinstance(complete_result, ActionErrorResult):
        print(f"   ✗ Complete signup failed: {complete_result.error_code}")
        return None

    assert isinstance(complete_result, CompleteSignupActionSuccessResult)
    print(f"   ✓ Signup complete!")
    print(f"   User ID: {complete_result.session.user_id}")
    print(f"   Session Token: {complete_result.session_token[:20]}...\n")

    return test_email, test_password


def test_signin_flow(email: str, password: str):
    """Test the complete signin flow."""
    print("=== Testing Signin Flow ===\n")

    client = HTTPClient("http://localhost:3777/")

    # Step 1: Create signin
    print("1. Creating signin...")
    signin_result = client.create_signin(email)
    if isinstance(signin_result, ActionErrorResult):
        print(f"   ✗ Signin failed: {signin_result.error_code}")
        return
    print(f"   ✓ Signin created: {signin_result.signin_token}\n")

    # Step 2: Verify password
    print("2. Verifying password...")
    verify_result = client.verify_signin_user_password(
        signin_result.signin_token, password
    )
    if isinstance(verify_result, ActionErrorResult):
        print(f"   ✗ Password verification failed: {verify_result.error_code}")
        return
    print("   ✓ Password verified\n")

    # Step 3: Complete signin
    print("3. Completing signin...")
    complete_result = client.complete_signin(signin_result.signin_token)
    if isinstance(complete_result, ActionErrorResult):
        print(f"   ✗ Complete signin failed: {complete_result.error_code}")
        return

    assert isinstance(complete_result, CompleteSigninActionSuccessResult)
    print(f"   ✓ Signin complete!")
    print(f"   User ID: {complete_result.session.user_id}")
    print(f"   Session Token: {complete_result.session_token[:20]}...\n")


def main():
    """Run the test flows."""
    try:
        # Test signup
        result = test_signup_flow()
        if result:
            email, password = result
            print("=" * 50)
            print()

            # Test signin
            test_signin_flow(email, password)

            print("=" * 50)
            print("✓ All tests passed!")
    except Exception as e:
        print(f"\n✗ Test failed with error: {e}")
        import traceback

        traceback.print_exc()


if __name__ == "__main__":
    main()
