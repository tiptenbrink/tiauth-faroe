from __future__ import annotations

import json
from collections.abc import Generator
from dataclasses import dataclass
from typing import Literal, cast

JSONValue = dict[str, "JSONValue"] | list["JSONValue"] | str | int | float | bool | None

JSONDict = dict[str, JSONValue]


def loads_typed(s: str) -> JSONValue:
    return cast(JSONValue, json.loads(s))


def validate_str_argument(argument: str, arguments_json_object: JSONDict) -> str:
    if argument not in arguments_json_object:
        raise ValueError(f"Missing '{argument}' field")

    value = arguments_json_object[argument]

    if not isinstance(value, str):
        raise ValueError(f"Invalid '{argument}' field: not a str")

    return value


def validate_dict_argument(argument: str, arguments_json_object: JSONDict) -> JSONDict:
    if argument not in arguments_json_object:
        raise ValueError(f"Missing '{argument}' field")

    value = arguments_json_object[argument]

    if not isinstance(value, dict):
        raise ValueError(f"Invalid '{argument}' field: not a dict")

    return value


def validate_int_argument(argument: str, arguments_json_object: JSONDict) -> int:
    if argument not in arguments_json_object:
        raise ValueError(f"Missing '{argument}' field")

    value = arguments_json_object[argument]

    if not isinstance(value, int):
        raise ValueError(f"Invalid '{argument}' field: not an int")

    return value


def validate_bool_argument(argument: str, arguments_json_object: JSONDict) -> bool:
    if argument not in arguments_json_object:
        raise ValueError(f"Missing '{argument}' field")

    value = arguments_json_object[argument]

    if not isinstance(value, bool):
        raise ValueError(f"Invalid '{argument}' field: not a bool")

    return value


def validate_optional_int_argument(
    argument: str, arguments_json_object: JSONDict
) -> int | None:
    if argument not in arguments_json_object:
        raise ValueError(f"Missing '{argument}' field")

    value = arguments_json_object[argument]

    if value is None:
        return None

    if not isinstance(value, int):
        raise ValueError(f"Invalid '{argument}' field: not an int or None")

    return value


@dataclass
class ActionSuccessResult:
    action_invocation_id: str
    ok: Literal[True] = True


@dataclass
class ActionParseSuccessResult:
    action_invocation_id: str
    values: JSONDict | None

    def as_success(self) -> ActionSuccessResult:
        return ActionSuccessResult(action_invocation_id=self.action_invocation_id)


@dataclass
class ActionErrorResult:
    action_invocation_id: str
    error_code: str
    ok: Literal[False] = False


ActionParseResult = ActionParseSuccessResult | ActionErrorResult
ActionResult = ActionSuccessResult | ActionErrorResult


def parse_action_invocation_response(
    response: JSONValue,
) -> ActionParseResult:
    if not isinstance(response, dict):
        raise ValueError("JSON value is not an object!")

    action_invocation_id = validate_str_argument("action_invocation_id", response)

    ok = validate_bool_argument("ok", response)

    if ok:
        if "values" in response:
            values = validate_dict_argument("values", response)
        else:
            values = None
        return ActionParseSuccessResult(action_invocation_id, values)
    else:
        error_code = validate_str_argument("error_code", response)
        return ActionErrorResult(action_invocation_id, error_code)


def expect_values(result: ActionParseSuccessResult) -> JSONDict:
    """Extract values from ActionParseSuccessResult, raising error if None."""
    if result.values is None:
        raise ValueError("Expected 'values' field in response but got None")
    return result.values


@dataclass
class Signup:
    id: str
    email_address: str
    email_address_verified: bool
    password_set: bool
    created_at: int
    expires_at: int


@dataclass
class CreateSignupActionSuccessResult:
    action_invocation_id: str
    signup: Signup
    signup_token: str
    ok: Literal[True] = True


@dataclass
class Session:
    id: str
    user_id: str
    created_at: int
    expires_at: int | None


@dataclass
class GetSessionActionSuccessResult:
    action_invocation_id: str
    session: Session
    ok: Literal[True] = True


@dataclass
class CompleteSignupActionSuccessResult:
    action_invocation_id: str
    session: Session
    session_token: str
    ok: Literal[True] = True


@dataclass
class Signin:
    id: str
    user_id: str
    user_first_factor_verified: bool
    created_at: int
    expires_at: int


@dataclass
class CreateSigninActionSuccessResult:
    action_invocation_id: str
    signin: Signin
    signin_token: str
    ok: Literal[True] = True


@dataclass
class CompleteSigninActionSuccessResult:
    action_invocation_id: str
    session: Session
    session_token: str
    ok: Literal[True] = True


def map_json_object_to_signup(json_object: JSONDict) -> Signup:
    """Convert a JSON object to a Signup instance."""
    signup_id = validate_str_argument("id", json_object)
    email_address = validate_str_argument("email_address", json_object)
    email_address_verified = validate_bool_argument(
        "email_address_verified", json_object
    )
    password_set = validate_bool_argument("password_set", json_object)

    created_at_timestamp = validate_int_argument("created_at", json_object)
    if created_at_timestamp < 0:
        raise ValueError("Invalid 'created_at' field: negative timestamp")

    expires_at_timestamp = validate_int_argument("expires_at", json_object)
    if expires_at_timestamp <= 0:
        raise ValueError("Invalid 'expires_at' field: non-positive timestamp")

    return Signup(
        id=signup_id,
        email_address=email_address,
        email_address_verified=email_address_verified,
        password_set=password_set,
        created_at=created_at_timestamp,
        expires_at=expires_at_timestamp,
    )


def map_json_object_to_session(json_object: JSONDict) -> Session:
    """Convert a JSON object to a Session instance."""

    session_id = validate_str_argument("id", json_object)
    user_id = validate_str_argument("user_id", json_object)

    created_at_timestamp = validate_int_argument("created_at", json_object)
    if created_at_timestamp < 0:
        raise ValueError("Invalid 'created_at' field: negative timestamp")

    expires_at_timestamp = validate_optional_int_argument("expires_at", json_object)
    if expires_at_timestamp is not None and expires_at_timestamp <= 0:
        raise ValueError("Invalid 'expires_at' field: non-positive timestamp")

    return Session(
        id=session_id,
        user_id=user_id,
        created_at=created_at_timestamp,
        expires_at=expires_at_timestamp,
    )


def map_json_object_to_signin(json_object: JSONDict) -> Signin:
    """Convert a JSON object to a Signin instance."""
    signin_id = validate_str_argument("id", json_object)
    user_id = validate_str_argument("user_id", json_object)
    user_first_factor_verified = validate_bool_argument(
        "user_first_factor_verified", json_object
    )

    created_at_timestamp = validate_int_argument("created_at", json_object)
    if created_at_timestamp < 0:
        raise ValueError("Invalid 'created_at' field: negative timestamp")

    expires_at_timestamp = validate_int_argument("expires_at", json_object)
    if expires_at_timestamp <= 0:
        raise ValueError("Invalid 'expires_at' field: non-positive timestamp")

    return Signin(
        id=signin_id,
        user_id=user_id,
        user_first_factor_verified=user_first_factor_verified,
        created_at=created_at_timestamp,
        expires_at=expires_at_timestamp,
    )


def create_signup(
    email_address: str,
) -> Generator[
    dict[str, JSONValue],
    ActionParseResult,
    CreateSignupActionSuccessResult | ActionErrorResult,
]:
    """Create a signup for the given email address."""
    arguments_object: JSONDict = {"email_address": email_address}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    values_json_object = expect_values(result)

    signup_dict = validate_dict_argument("signup", values_json_object)
    signup = map_json_object_to_signup(signup_dict)

    signup_token = validate_str_argument("signup_token", values_json_object)

    return CreateSignupActionSuccessResult(
        action_invocation_id=result.action_invocation_id,
        signup=signup,
        signup_token=signup_token,
    )


def get_session(
    session_token: str,
) -> Generator[
    dict[str, JSONValue],
    ActionParseResult,
    GetSessionActionSuccessResult | ActionErrorResult,
]:
    """Get session information for the given session token."""
    arguments_object: JSONDict = {"session_token": session_token}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    values_json_object = expect_values(result)

    session_dict = validate_dict_argument("session", values_json_object)
    session = map_json_object_to_session(session_dict)

    return GetSessionActionSuccessResult(
        action_invocation_id=result.action_invocation_id, session=session
    )


def verify_signup_email_address_verification_code(
    signup_token: str, email_address_verification_code: str
) -> Generator[
    dict[str, JSONValue],
    ActionParseResult,
    ActionSuccessResult | ActionErrorResult,
]:
    """Verify the email address verification code for a signup."""
    arguments_object: JSONDict = {
        "signup_token": signup_token,
        "email_address_verification_code": email_address_verification_code,
    }

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    return result.as_success()


def set_signup_password(
    signup_token: str, password: str
) -> Generator[
    dict[str, JSONValue], ActionParseResult, ActionSuccessResult | ActionErrorResult
]:
    """Set the password for a signup."""
    arguments_object: JSONDict = {"signup_token": signup_token, "password": password}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    return result.as_success()


def complete_signup(
    signup_token: str,
) -> Generator[
    dict[str, JSONValue],
    ActionParseResult,
    CompleteSignupActionSuccessResult | ActionErrorResult,
]:
    """Complete the signup process and return session information."""
    arguments_object: JSONDict = {"signup_token": signup_token}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    values_json_object = expect_values(result)

    session_dict = validate_dict_argument("session", values_json_object)
    session = map_json_object_to_session(session_dict)

    session_token = validate_str_argument("session_token", values_json_object)

    return CompleteSignupActionSuccessResult(
        action_invocation_id=result.action_invocation_id,
        session=session,
        session_token=session_token,
    )


def create_signin(
    user_email_address: str,
) -> Generator[
    dict[str, JSONValue],
    ActionParseResult,
    CreateSigninActionSuccessResult | ActionErrorResult,
]:
    """Create a signin for the given email address."""
    arguments_object: JSONDict = {"user_email_address": user_email_address}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    values_json_object = expect_values(result)

    signin_dict = validate_dict_argument("signin", values_json_object)
    signin = map_json_object_to_signin(signin_dict)

    signin_token = validate_str_argument("signin_token", values_json_object)

    return CreateSigninActionSuccessResult(
        action_invocation_id=result.action_invocation_id,
        signin=signin,
        signin_token=signin_token,
    )


def verify_signin_user_password(
    signin_token: str, password: str
) -> Generator[
    dict[str, JSONValue], ActionParseResult, ActionSuccessResult | ActionErrorResult
]:
    """Verify the user password for a signin."""
    arguments_object: JSONDict = {"signin_token": signin_token, "password": password}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    return ActionSuccessResult(
        ok=True, action_invocation_id=result.action_invocation_id
    )


def complete_signin(
    signin_token: str,
) -> Generator[
    dict[str, JSONValue],
    ActionParseResult,
    CompleteSigninActionSuccessResult | ActionErrorResult,
]:
    """Complete the signin process and return session information."""
    arguments_object: JSONDict = {"signin_token": signin_token}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    values_json_object = expect_values(result)

    session_dict = validate_dict_argument("session", values_json_object)
    session = map_json_object_to_session(session_dict)

    session_token = validate_str_argument("session_token", values_json_object)

    return CompleteSigninActionSuccessResult(
        action_invocation_id=result.action_invocation_id,
        session=session,
        session_token=session_token,
    )
