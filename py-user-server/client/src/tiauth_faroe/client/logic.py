from __future__ import annotations

import json
from collections.abc import Generator
from dataclasses import dataclass
from typing import cast

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
    values: JSONDict


@dataclass
class ActionErrorResult:
    """Represents an error result from an action."""

    action_invocation_id: str
    error_code: str


ActionResult = ActionSuccessResult | ActionErrorResult


def parse_action_invocation_response(response: JSONValue) -> ActionResult:
    if not isinstance(response, dict):
        raise ValueError("JSON value is not an object!")

    action_invocation_id = validate_str_argument("action_invocation_id", response)

    ok = validate_bool_argument("ok", response)

    if ok:
        values = validate_dict_argument("values", response)
        return ActionSuccessResult(action_invocation_id, values)
    else:
        error_code = validate_str_argument("error_code", response)
        return ActionErrorResult(action_invocation_id, error_code)


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


def get_session(
    session_token: str,
) -> Generator[
    dict[str, JSONValue],
    ActionResult,
    GetSessionActionSuccessResult | ActionErrorResult,
]:
    """Get session information for the given session token."""
    arguments_object: JSONDict = {"session_token": session_token}

    result = yield arguments_object
    if isinstance(result, ActionErrorResult):
        return result

    values_json_object = result.values

    session_dict = validate_dict_argument("session", values_json_object)
    session = map_json_object_to_session(session_dict)

    return GetSessionActionSuccessResult(
        action_invocation_id=result.action_invocation_id, session=session
    )
