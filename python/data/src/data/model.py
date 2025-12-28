from dataclasses import dataclass

import sqlalchemy as sqla

# Helps name constraints
convention = {
    "ix": "ix_%(column_0_label)s",
    "uq": "uq_%(table_name)s_%(column_0_name)s",
    "ck": "ck_%(table_name)s_%(constraint_name)s",
    "fk": "fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s",
    "pk": "pk_%(table_name)s",
}

metadata = sqla.MetaData(naming_convention=convention)


@dataclass(frozen=True)
class UserTable:
    NAME = "user"
    ID = "user_id"
    EMAIL = "email"
    DISPLAY_NAME = "display_name"
    PASSWORD_HASH = "password_hash"
    PASSWORD_SALT = "password_salt"
    PASSWORD_HASH_ALGORITHM_ID = "password_hash_algorithm_id"
    DISABLED = "disabled"
    EMAIL_ADDRESS_COUNTER = "email_address_counter"
    PASSWORD_HASH_COUNTER = "password_hash_counter"
    DISABLED_COUNTER = "disabled_counter"
    SESSIONS_COUNTER = "sessions_counter"


users = sqla.Table(
    UserTable.NAME,
    metadata,
    sqla.Column(UserTable.ID, sqla.Text, primary_key=True),
    sqla.Column(UserTable.EMAIL, sqla.Text, unique=True, nullable=False, index=True),
    sqla.Column(UserTable.DISPLAY_NAME, sqla.Text, nullable=False),
    sqla.Column(UserTable.PASSWORD_HASH, sqla.LargeBinary, nullable=False),
    sqla.Column(UserTable.PASSWORD_SALT, sqla.LargeBinary, nullable=False),
    sqla.Column(UserTable.PASSWORD_HASH_ALGORITHM_ID, sqla.Text, nullable=False),
    sqla.Column(UserTable.DISABLED, sqla.Integer, nullable=False, default=0),
    sqla.Column(
        UserTable.EMAIL_ADDRESS_COUNTER, sqla.Integer, nullable=False, default=0
    ),
    sqla.Column(
        UserTable.PASSWORD_HASH_COUNTER, sqla.Integer, nullable=False, default=0
    ),
    sqla.Column(UserTable.DISABLED_COUNTER, sqla.Integer, nullable=False, default=0),
    sqla.Column(UserTable.SESSIONS_COUNTER, sqla.Integer, nullable=False, default=0),
    sqlite_with_rowid=False,
    sqlite_strict=True,
)
