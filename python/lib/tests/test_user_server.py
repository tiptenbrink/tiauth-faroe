from typing import Any
import pytest
import json
import base64
from tiauth_faroe.user_server import (
    User,
    Effect,
    EffectResult,
    SyncServer,
    CreateUserEffect,
    GetUserEffect,
    GetUserByEmailAddressEffect,
    UpdateUserEmailAddressEffect,
    UpdateUserPasswordHashEffect,
    IncrementUserSessionsCounterEffect,
    DeleteUserEffect,
    ActionError,
    handle_request_sync,
    process_request_gen,
)


def make_request(action: str, **arguments) -> dict[str, Any]:
    """Helper function to create JSON request bodies"""
    return {"action": action, "arguments": arguments}


class DictDatabase(SyncServer):
    """Simple dict-based database for testing"""

    def __init__(self):
        self.users: dict[str, User] = {}
        self.email_to_user_id: dict[str, str] = {}
        self.next_user_id = 1

    def execute_effect(self, effect: Effect) -> EffectResult:
        if isinstance(effect, CreateUserEffect):
            # Check if email already exists (unique constraint)
            if effect.email_address in self.email_to_user_id:
                return ActionError("email_address_already_used")

            user_id = f"user_{self.next_user_id}"
            self.next_user_id += 1

            user = User(
                id=user_id,
                email_address=effect.email_address,
                password_hash=effect.password_hash,
                password_hash_algorithm_id=effect.password_hash_algorithm_id,
                password_salt=effect.password_salt,
                disabled=False,
                display_name="",
                email_address_counter=0,
                password_hash_counter=0,
                disabled_counter=0,
                sessions_counter=0,
            )

            self.users[user_id] = user
            self.email_to_user_id[effect.email_address] = user_id
            return user

        elif isinstance(effect, GetUserEffect):
            if effect.user_id not in self.users:
                return ActionError("user_not_found")
            return self.users[effect.user_id]

        elif isinstance(effect, GetUserByEmailAddressEffect):
            if effect.email_address not in self.email_to_user_id:
                return ActionError("user_not_found")
            user_id = self.email_to_user_id[effect.email_address]
            return self.users[user_id]

        elif isinstance(effect, UpdateUserEmailAddressEffect):
            if effect.user_id not in self.users:
                return ActionError("user_not_found")

            user = self.users[effect.user_id]

            # Update email mappings
            del self.email_to_user_id[user.email_address]
            self.email_to_user_id[effect.email_address] = effect.user_id

            # Update user
            user.email_address = effect.email_address
            user.email_address_counter += 1
            return None

        elif isinstance(effect, UpdateUserPasswordHashEffect):
            if effect.user_id not in self.users:
                return ActionError("user_not_found")

            user = self.users[effect.user_id]

            user.password_hash = effect.password_hash
            user.password_hash_algorithm_id = effect.password_hash_algorithm_id
            user.password_salt = effect.password_salt
            user.password_hash_counter += 1
            return None

        elif isinstance(effect, IncrementUserSessionsCounterEffect):
            if effect.user_id not in self.users:
                return ActionError("user_not_found")

            user = self.users[effect.user_id]
            user.sessions_counter += 1
            return None

        elif isinstance(effect, DeleteUserEffect):
            if effect.user_id not in self.users:
                return ActionError("user_not_found")

            user = self.users[effect.user_id]
            del self.email_to_user_id[user.email_address]
            del self.users[effect.user_id]
            return None

        else:
            raise ValueError(f"Unknown effect type: {type(effect)}")


@pytest.fixture
def db():
    return DictDatabase()


class TestCreateUser:
    def test_create_user_success(self, db):
        request_body = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"hashed_password").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt123").decode(),
        )

        result = handle_request_sync(request_body, db)

        assert result.error is None
        response = json.loads(result.response_json)
        assert response["ok"] is True
        assert "action_invocation_id" in response
        assert response["values"]["user"]["email_address"] == "test@example.com"
        assert response["values"]["user"]["password_hash_algorithm_id"] == "bcrypt"
        assert not response["values"]["user"]["disabled"]
        assert response["values"]["user"]["display_name"] == ""
        assert response["values"]["user"]["email_address_counter"] == 0
        assert response["values"]["user"]["password_hash_counter"] == 0

    def test_create_user_duplicate_email(self, db):
        request_body1 = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"password1").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt1").decode(),
        )
        handle_request_sync(request_body1, db)

        # Second user with same email
        request_body2 = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"password2").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt2").decode(),
        )

        result = handle_request_sync(request_body2, db)

        response = json.loads(result.response_json)
        assert response["ok"] is False
        assert response["error_code"] == "email_address_already_used"


class TestGetUser:
    def test_get_user_success(self, db):
        create_request = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"password").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt").decode(),
        )
        create_result = handle_request_sync(create_request, db)
        create_response = json.loads(create_result.response_json)
        user_id = create_response["values"]["user"]["id"]

        get_request = make_request("get_user", user_id=user_id)
        result = handle_request_sync(get_request, db)

        assert result.error is None
        response = json.loads(result.response_json)
        assert response["ok"] is True
        assert response["values"]["user"]["email_address"] == "test@example.com"
        assert response["values"]["user"]["id"] == user_id

    def test_get_user_not_found(self, db):
        request_body = make_request("get_user", user_id="nonexistent")
        result = handle_request_sync(request_body, db)

        response = json.loads(result.response_json)
        assert response["ok"] is False
        assert response["error_code"] == "user_not_found"


class TestGetUserByEmailAddress:
    def test_get_user_by_email_success(self, db):
        create_request = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"password").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt").decode(),
        )
        handle_request_sync(create_request, db)

        get_request = make_request(
            "get_user_by_email_address", email_address="test@example.com"
        )
        result = handle_request_sync(get_request, db)

        assert result.error is None
        response = json.loads(result.response_json)
        assert response["ok"] is True
        assert response["values"]["user"]["email_address"] == "test@example.com"

    def test_get_user_by_email_not_found(self, db):
        request_body = make_request(
            "get_user_by_email_address", email_address="nonexistent@example.com"
        )
        result = handle_request_sync(request_body, db)

        response = json.loads(result.response_json)
        assert response["ok"] is False
        assert response["error_code"] == "user_not_found"


class TestUpdateUserEmailAddress:
    def test_update_email_success(self, db):
        create_request = make_request(
            "create_user",
            email_address="old@example.com",
            password_hash=base64.b64encode(b"password").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt").decode(),
        )
        create_result = handle_request_sync(create_request, db)
        create_response = json.loads(create_result.response_json)
        user_id = create_response["values"]["user"]["id"]

        update_request = make_request(
            "update_user_email_address",
            user_id=user_id,
            email_address="new@example.com",
            user_email_address_counter=0,  # Should be 0 initially per schema
        )
        result = handle_request_sync(update_request, db)

        assert result.error is None
        response = json.loads(result.response_json)
        assert response["ok"] is True

        # Verify user was updated
        get_request = make_request("get_user", user_id=user_id)
        get_result = handle_request_sync(get_request, db)
        get_response = json.loads(get_result.response_json)
        assert get_response["values"]["user"]["email_address"] == "new@example.com"
        assert get_response["values"]["user"]["email_address_counter"] == 1

    def test_update_email_user_not_found(self, db):
        update_request = make_request(
            "update_user_email_address",
            user_id="nonexistent",
            email_address="new@example.com",
            user_email_address_counter=0,
        )
        result = handle_request_sync(update_request, db)

        response = json.loads(result.response_json)
        assert response["ok"] is False
        assert response["error_code"] == "user_not_found"


class TestUpdateUserPasswordHash:
    def test_update_password_success(self, db):
        create_request = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"old_password").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"old_salt").decode(),
        )
        create_result = handle_request_sync(create_request, db)
        create_response = json.loads(create_result.response_json)
        user_id = create_response["values"]["user"]["id"]

        update_request = make_request(
            "update_user_password_hash",
            user_id=user_id,
            password_hash=base64.b64encode(b"new_password").decode(),
            password_hash_algorithm_id="argon2",
            password_salt=base64.b64encode(b"new_salt").decode(),
            user_password_hash_counter=0,  # Should be 0 initially per schema
        )
        result = handle_request_sync(update_request, db)

        assert result.error is None
        response = json.loads(result.response_json)
        assert response["ok"] is True

        # Verify password was updated
        get_request = make_request("get_user", user_id=user_id)
        get_result = handle_request_sync(get_request, db)
        get_response = json.loads(get_result.response_json)
        assert get_response["values"]["user"]["password_hash_algorithm_id"] == "argon2"
        assert get_response["values"]["user"]["password_hash_counter"] == 1


class TestIncrementUserSessionsCounter:
    def test_increment_sessions_success(self, db):
        create_request = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"password").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt").decode(),
        )
        create_result = handle_request_sync(create_request, db)
        create_response = json.loads(create_result.response_json)
        user_id = create_response["values"]["user"]["id"]

        increment_request = make_request(
            "increment_user_sessions_counter",
            user_id=user_id,
            user_sessions_counter=0,  # Should be 0 initially per schema
        )
        result = handle_request_sync(increment_request, db)

        assert result.error is None
        response = json.loads(result.response_json)
        assert response["ok"] is True

        # Verify sessions counter was incremented
        get_request = make_request("get_user", user_id=user_id)
        get_result = handle_request_sync(get_request, db)
        get_response = json.loads(get_result.response_json)
        assert get_response["values"]["user"]["sessions_counter"] == 1


class TestDeleteUser:
    def test_delete_user_success(self, db):
        create_request = make_request(
            "create_user",
            email_address="test@example.com",
            password_hash=base64.b64encode(b"password").decode(),
            password_hash_algorithm_id="bcrypt",
            password_salt=base64.b64encode(b"salt").decode(),
        )
        create_result = handle_request_sync(create_request, db)
        create_response = json.loads(create_result.response_json)
        user_id = create_response["values"]["user"]["id"]

        delete_request = make_request("delete_user", user_id=user_id)
        result = handle_request_sync(delete_request, db)

        assert result.error is None
        response = json.loads(result.response_json)
        assert response["ok"] is True

        # Verify user was deleted
        get_request = make_request("get_user", user_id=user_id)
        get_result = handle_request_sync(get_request, db)
        get_response = json.loads(get_result.response_json)
        assert get_response["ok"] is False
        assert get_response["error_code"] == "user_not_found"

    def test_delete_user_not_found(self, db):
        request_body = make_request("delete_user", user_id="nonexistent")
        result = handle_request_sync(request_body, db)

        response = json.loads(result.response_json)
        assert response["ok"] is False
        assert response["error_code"] == "user_not_found"


class TestInvalidActions:
    def test_unknown_action(self, db):
        request_body = make_request("unknown_action")

        # Unknown actions cause the generator to return immediately
        generator = process_request_gen(request_body)
        try:
            next(generator)
            assert False, "Expected StopIteration"
        except StopIteration as e:
            result = e.value
            assert result.error == "Unknown action unknown_action"

    def test_invalid_object(self, db):
        with pytest.raises(ValueError, match="Invalid request object"):
            handle_request_sync("invalid json", db)

    def test_missing_action_field(self, db):
        request = {"arguments": {}}
        with pytest.raises(ValueError, match="Missing or invalid 'action' field"):
            handle_request_sync(request, db)

    def test_missing_arguments_field(self, db):
        request = {"action": "create_user"}
        with pytest.raises(ValueError, match="Missing or invalid 'arguments' field"):
            handle_request_sync(request, db)
