import sqlite3
from sqlalchemy import text, Engine
from sqlalchemy.exc import IntegrityError, SQLAlchemyError
from tiauth_faroe.user_server import (
    Effect,
    EffectResult,
    User,
    ActionError,
    CreateUserEffect,
    GetUserEffect,
    GetUserByEmailAddressEffect,
    UpdateUserEmailAddressEffect,
    UpdateUserPasswordHashEffect,
    IncrementUserSessionsCounterEffect,
    DeleteUserEffect,
    SyncServer,
)
from .model import UserTable


def check_integrity_error(e: IntegrityError, column: str, category: str) -> bool:
    """For column 'email' in table 'user', 'user.email' would be the column. For category, at least 'unique' works."""
    orig = e.orig
    assert isinstance(orig, sqlite3.IntegrityError)
    assert isinstance(orig.args[0], str)
    return column in orig.args[0] and category in orig.args[0].lower()


def create_user(engine: Engine, effect: CreateUserEffect) -> EffectResult:
    try:
        with engine.begin() as conn:
            email_prefix = effect.email_address.split("@")[0]

            # Generate a unique user ID by finding the highest existing numeric prefix
            max_id_stmt = text(f"""
                SELECT CAST(SUBSTR({UserTable.ID}, 1, INSTR({UserTable.ID}, '_') - 1) AS INTEGER) as max_prefix
                FROM {UserTable.NAME}
                WHERE {UserTable.ID} LIKE '%_{email_prefix}'
                ORDER BY max_prefix DESC
                LIMIT 1
            """)
            max_id_result = conn.execute(max_id_stmt)
            max_id_row = max_id_result.first()

            if max_id_row is None or max_id_row[0] is None:
                next_id = 1
            else:
                next_id = max_id_row[0] + 1

            user_id = f"{next_id}_{email_prefix}"

            stmt = text(f"""
                INSERT INTO {UserTable.NAME} (
                    {UserTable.ID}, {UserTable.EMAIL},
                    {UserTable.DISPLAY_NAME},
                    {UserTable.PASSWORD_HASH}, {UserTable.PASSWORD_HASH_ALGORITHM_ID},
                    {UserTable.PASSWORD_SALT}, {UserTable.DISABLED},
                    {UserTable.EMAIL_ADDRESS_COUNTER}, {UserTable.PASSWORD_HASH_COUNTER},
                    {UserTable.DISABLED_COUNTER}, {UserTable.SESSIONS_COUNTER}
                ) VALUES (
                    :user_id, :email_address,
                    :display_name,
                    :password_hash, :password_hash_algorithm_id, :password_salt,
                    :disabled, :email_address_counter, :password_hash_counter,
                    :disabled_counter, :sessions_counter
                )
            """)

            conn.execute(
                stmt,
                {
                    "user_id": user_id,
                    "email_address": effect.email_address,
                    "display_name": "",
                    "password_hash": effect.password_hash,
                    "password_hash_algorithm_id": effect.password_hash_algorithm_id,
                    "password_salt": effect.password_salt,
                    "disabled": 0,
                    "email_address_counter": 0,
                    "password_hash_counter": 0,
                    "disabled_counter": 0,
                    "sessions_counter": 0,
                },
            )

    except IntegrityError as e:
        if check_integrity_error(e, "user.email", "unique"):
            return ActionError("email_address_already_used")
        else:
            return ActionError("unexpected_error")
    except SQLAlchemyError:
        return ActionError("unexpected_error")

    return User(
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


def get_user(engine: Engine, effect: GetUserEffect) -> EffectResult:
    try:
        with engine.connect() as conn:
            stmt = text(f"""
                SELECT {UserTable.EMAIL}, {UserTable.DISPLAY_NAME},
                       {UserTable.PASSWORD_HASH}, {UserTable.PASSWORD_HASH_ALGORITHM_ID},
                       {UserTable.PASSWORD_SALT}, {UserTable.DISABLED},
                       {UserTable.EMAIL_ADDRESS_COUNTER}, {UserTable.PASSWORD_HASH_COUNTER},
                       {UserTable.DISABLED_COUNTER}, {UserTable.SESSIONS_COUNTER}
                FROM {UserTable.NAME}
                WHERE {UserTable.ID} = :user_id
            """)

            result = conn.execute(stmt, {"user_id": effect.user_id})
            row = result.first()

    except SQLAlchemyError:
        return ActionError("unexpected_error")

    if row is None:
        return ActionError("user_not_found")

    return User(
        id=effect.user_id,
        email_address=getattr(row, UserTable.EMAIL),
        password_hash=getattr(row, UserTable.PASSWORD_HASH),
        password_hash_algorithm_id=getattr(row, UserTable.PASSWORD_HASH_ALGORITHM_ID),
        password_salt=getattr(row, UserTable.PASSWORD_SALT),
        disabled=bool(getattr(row, UserTable.DISABLED)),
        display_name=getattr(row, UserTable.DISPLAY_NAME),
        email_address_counter=getattr(row, UserTable.EMAIL_ADDRESS_COUNTER),
        password_hash_counter=getattr(row, UserTable.PASSWORD_HASH_COUNTER),
        disabled_counter=getattr(row, UserTable.DISABLED_COUNTER),
        sessions_counter=getattr(row, UserTable.SESSIONS_COUNTER),
    )


def get_user_by_email_address(
    engine: Engine, effect: GetUserByEmailAddressEffect
) -> EffectResult:
    try:
        with engine.connect() as conn:
            stmt = text(f"""
                SELECT {UserTable.ID}, {UserTable.DISPLAY_NAME},
                       {UserTable.PASSWORD_HASH}, {UserTable.PASSWORD_HASH_ALGORITHM_ID},
                       {UserTable.PASSWORD_SALT}, {UserTable.DISABLED},
                       {UserTable.EMAIL_ADDRESS_COUNTER}, {UserTable.PASSWORD_HASH_COUNTER},
                       {UserTable.DISABLED_COUNTER}, {UserTable.SESSIONS_COUNTER}
                FROM {UserTable.NAME}
                WHERE {UserTable.EMAIL} = :email_address
            """)

            result = conn.execute(stmt, {"email_address": effect.email_address})
            row = result.first()

    except SQLAlchemyError:
        return ActionError("unexpected_error")

    if row is None:
        return ActionError("user_not_found")

    return User(
        id=getattr(row, UserTable.ID),
        email_address=effect.email_address,
        password_hash=getattr(row, UserTable.PASSWORD_HASH),
        password_hash_algorithm_id=getattr(row, UserTable.PASSWORD_HASH_ALGORITHM_ID),
        password_salt=getattr(row, UserTable.PASSWORD_SALT),
        disabled=bool(getattr(row, UserTable.DISABLED)),
        display_name=getattr(row, UserTable.DISPLAY_NAME),
        email_address_counter=getattr(row, UserTable.EMAIL_ADDRESS_COUNTER),
        password_hash_counter=getattr(row, UserTable.PASSWORD_HASH_COUNTER),
        disabled_counter=getattr(row, UserTable.DISABLED_COUNTER),
        sessions_counter=getattr(row, UserTable.SESSIONS_COUNTER),
    )


def update_user_email_address(
    engine: Engine, effect: UpdateUserEmailAddressEffect
) -> EffectResult:
    try:
        with engine.begin() as conn:
            stmt = text(f"""
                UPDATE {UserTable.NAME}
                SET {UserTable.EMAIL} = :email_address,
                    {UserTable.EMAIL_ADDRESS_COUNTER} = {UserTable.EMAIL_ADDRESS_COUNTER} + 1
                WHERE {UserTable.ID} = :user_id
                  AND {UserTable.EMAIL_ADDRESS_COUNTER} = :user_email_address_counter
            """)

            result = conn.execute(
                stmt,
                {
                    "email_address": effect.email_address,
                    "user_id": effect.user_id,
                    "user_email_address_counter": effect.user_email_address_counter,
                },
            )

    except SQLAlchemyError:
        return ActionError("unexpected_error")

    if result.rowcount == 0:
        return ActionError("user_not_found")

    return None


def update_user_password_hash(
    engine: Engine, effect: UpdateUserPasswordHashEffect
) -> EffectResult:
    try:
        with engine.begin() as conn:
            stmt = text(f"""
                UPDATE {UserTable.NAME}
                SET {UserTable.PASSWORD_HASH} = :password_hash,
                    {UserTable.PASSWORD_HASH_ALGORITHM_ID} = :password_hash_algorithm_id,
                    {UserTable.PASSWORD_SALT} = :password_salt,
                    {UserTable.PASSWORD_HASH_COUNTER} = {UserTable.PASSWORD_HASH_COUNTER} + 1
                WHERE {UserTable.ID} = :user_id
                  AND {UserTable.PASSWORD_HASH_COUNTER} = :user_password_hash_counter
            """)

            result = conn.execute(
                stmt,
                {
                    "password_hash": effect.password_hash,
                    "password_hash_algorithm_id": effect.password_hash_algorithm_id,
                    "password_salt": effect.password_salt,
                    "user_id": effect.user_id,
                    "user_password_hash_counter": effect.user_password_hash_counter,
                },
            )

    except SQLAlchemyError:
        return ActionError("unexpected_error")

    if result.rowcount == 0:
        return ActionError("user_not_found")

    return None


def increment_user_sessions_counter(
    engine: Engine, effect: IncrementUserSessionsCounterEffect
) -> EffectResult:
    try:
        with engine.begin() as conn:
            stmt = text(f"""
                UPDATE {UserTable.NAME}
                SET {UserTable.SESSIONS_COUNTER} = {UserTable.SESSIONS_COUNTER} + 1
                WHERE {UserTable.ID} = :user_id
                  AND {UserTable.SESSIONS_COUNTER} = :user_sessions_counter
            """)

            result = conn.execute(
                stmt,
                {
                    "user_id": effect.user_id,
                    "user_sessions_counter": effect.user_sessions_counter,
                },
            )

    except SQLAlchemyError:
        return ActionError("unexpected_error")

    if result.rowcount == 0:
        return ActionError("user_not_found")

    return None


def delete_user(engine: Engine, effect: DeleteUserEffect) -> EffectResult:
    try:
        with engine.begin() as conn:
            stmt = text(f"""
                DELETE FROM {UserTable.NAME}
                WHERE {UserTable.ID} = :user_id
            """)

            result = conn.execute(stmt, {"user_id": effect.user_id})

    except SQLAlchemyError:
        return ActionError("unexpected_error")

    if result.rowcount == 0:
        return ActionError("user_not_found")

    return None


class SqliteSyncServer(SyncServer):
    def __init__(self, engine: Engine):
        self.engine = engine

    def execute_effect(self, effect: Effect) -> EffectResult:
        print(f"effect:\n{effect}\n")
        if isinstance(effect, CreateUserEffect):
            return create_user(self.engine, effect)
        elif isinstance(effect, GetUserEffect):
            return get_user(self.engine, effect)
        elif isinstance(effect, GetUserByEmailAddressEffect):
            return get_user_by_email_address(self.engine, effect)
        elif isinstance(effect, UpdateUserEmailAddressEffect):
            return update_user_email_address(self.engine, effect)
        elif isinstance(effect, UpdateUserPasswordHashEffect):
            return update_user_password_hash(self.engine, effect)
        elif isinstance(effect, IncrementUserSessionsCounterEffect):
            return increment_user_sessions_counter(self.engine, effect)
        elif isinstance(effect, DeleteUserEffect):
            return delete_user(self.engine, effect)
        else:
            raise ValueError(f"Unknown effect type: {type(effect)}")


def clear_all_users(engine: Engine):
    with engine.begin() as conn:
        delete_stmt = text(f"""
            DELETE FROM {UserTable.NAME}
        """)
        conn.execute(delete_stmt)
