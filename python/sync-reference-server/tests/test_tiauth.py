"""Basic tests for sync-reference-server."""

import pytest
from sqlalchemy import create_engine

from sync_reference_server.data.model import UserTable, metadata
from sync_reference_server.data.queries import SqliteSyncServer, clear_all_users
from tiauth_faroe.user_server import (
    CreateUserEffect,
    GetUserEffect,
    GetUserByEmailAddressEffect,
    DeleteUserEffect,
    User,
    ActionError,
    generate_action_invocation_id,
)


class TestUserTable:
    """Test UserTable constants."""

    def test_table_name(self):
        assert UserTable.NAME == "user"

    def test_column_names(self):
        assert UserTable.ID == "user_id"
        assert UserTable.EMAIL == "email"
        assert UserTable.DISPLAY_NAME == "display_name"
        assert UserTable.PASSWORD_HASH == "password_hash"


class TestSqliteSyncServer:
    """Test SqliteSyncServer with in-memory database."""

    @pytest.fixture
    def engine(self):
        """Create an in-memory SQLite database."""
        engine = create_engine("sqlite:///:memory:")
        metadata.create_all(engine)
        return engine

    @pytest.fixture
    def server(self, engine):
        """Create a SqliteSyncServer instance."""
        return SqliteSyncServer(engine)

    def test_create_user(self, server):
        """Test creating a user."""
        effect = CreateUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            email_address="test@example.com",
            password_hash=b"hash123",
            password_hash_algorithm_id="argon2id",
            password_salt=b"salt456",
        )
        result = server.execute_effect(effect)

        assert isinstance(result, User)
        assert result.email_address == "test@example.com"
        assert result.id == "1_test"

    def test_create_duplicate_email_fails(self, server):
        """Test that creating a user with duplicate email fails."""
        effect = CreateUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            email_address="test@example.com",
            password_hash=b"hash123",
            password_hash_algorithm_id="argon2id",
            password_salt=b"salt456",
        )
        server.execute_effect(effect)

        # Try to create another user with the same email
        effect2 = CreateUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            email_address="test@example.com",
            password_hash=b"hash123",
            password_hash_algorithm_id="argon2id",
            password_salt=b"salt456",
        )
        result = server.execute_effect(effect2)
        assert isinstance(result, ActionError)
        assert result.error_code == "email_address_already_used"

    def test_get_user_by_email(self, server):
        """Test getting a user by email address."""
        # Create a user first
        create_effect = CreateUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            email_address="find@example.com",
            password_hash=b"hash",
            password_hash_algorithm_id="argon2id",
            password_salt=b"salt",
        )
        created = server.execute_effect(create_effect)
        assert isinstance(created, User)

        # Get by email
        get_effect = GetUserByEmailAddressEffect(
            action_invocation_id=generate_action_invocation_id(),
            email_address="find@example.com",
        )
        result = server.execute_effect(get_effect)

        assert isinstance(result, User)
        assert result.email_address == "find@example.com"
        assert result.id == created.id

    def test_get_nonexistent_user(self, server):
        """Test getting a user that doesn't exist."""
        effect = GetUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            user_id="nonexistent",
        )
        result = server.execute_effect(effect)

        assert isinstance(result, ActionError)
        assert result.error_code == "user_not_found"

    def test_delete_user(self, server):
        """Test deleting a user."""
        # Create a user first
        create_effect = CreateUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            email_address="delete@example.com",
            password_hash=b"hash",
            password_hash_algorithm_id="argon2id",
            password_salt=b"salt",
        )
        created = server.execute_effect(create_effect)
        assert isinstance(created, User)

        # Delete the user
        delete_effect = DeleteUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            user_id=created.id,
        )
        result = server.execute_effect(delete_effect)
        assert result is None

        # Verify user is gone
        get_effect = GetUserEffect(
            action_invocation_id=generate_action_invocation_id(),
            user_id=created.id,
        )
        result = server.execute_effect(get_effect)
        assert isinstance(result, ActionError)
        assert result.error_code == "user_not_found"

    def test_clear_all_users(self, engine, server):
        """Test clearing all users."""
        # Create some users
        for i in range(3):
            effect = CreateUserEffect(
                action_invocation_id=generate_action_invocation_id(),
                email_address=f"user{i}@example.com",
                password_hash=b"hash",
                password_hash_algorithm_id="argon2id",
                password_salt=b"salt",
            )
            server.execute_effect(effect)

        # Clear all
        clear_all_users(engine)

        # Verify all users are gone
        effect = GetUserByEmailAddressEffect(
            action_invocation_id=generate_action_invocation_id(),
            email_address="user0@example.com",
        )
        result = server.execute_effect(effect)
        assert isinstance(result, ActionError)
        assert result.error_code == "user_not_found"
