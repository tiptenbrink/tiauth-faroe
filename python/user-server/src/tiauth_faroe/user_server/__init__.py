import secrets
import json
import base64
import binascii
from dataclasses import dataclass
from typing import Generator, Union, Protocol, Dict, Any


@dataclass
class User:
    id: str
    email_address: str
    password_hash: bytes
    password_hash_algorithm_id: str
    password_salt: bytes
    disabled: bool
    display_name: str
    email_address_counter: int
    password_hash_counter: int
    disabled_counter: int
    sessions_counter: int


@dataclass
class CreateUserEffect:
    action_invocation_id: str
    email_address: str
    password_hash: bytes
    password_hash_algorithm_id: str
    password_salt: bytes


@dataclass
class GetUserEffect:
    action_invocation_id: str
    user_id: str


@dataclass
class GetUserByEmailAddressEffect:
    action_invocation_id: str
    email_address: str


@dataclass
class UpdateUserEmailAddressEffect:
    action_invocation_id: str
    user_id: str
    email_address: str
    user_email_address_counter: int


@dataclass
class UpdateUserPasswordHashEffect:
    action_invocation_id: str
    user_id: str
    password_hash: bytes
    password_hash_algorithm_id: str
    password_salt: bytes
    user_password_hash_counter: int


@dataclass
class IncrementUserSessionsCounterEffect:
    action_invocation_id: str
    user_id: str
    user_sessions_counter: int


@dataclass
class DeleteUserEffect:
    action_invocation_id: str
    user_id: str


Effect = Union[
    CreateUserEffect,
    GetUserEffect,
    GetUserByEmailAddressEffect,
    UpdateUserEmailAddressEffect,
    UpdateUserPasswordHashEffect,
    IncrementUserSessionsCounterEffect,
    DeleteUserEffect,
]


@dataclass
class ServerResult:
    response_json: str
    error: str | None = None


class ActionError(Exception):
    def __init__(self, error_code: str):
        self.error_code = error_code
        super().__init__(error_code)


EffectResult = Union[User, ActionError, None]


def generate_action_invocation_id() -> str:
    alphabet = "abcdefghjkmnopqrstuvwxyz23456789"
    bytes_array = secrets.token_bytes(24)
    id_str = ""
    for byte in bytes_array:
        id_str += alphabet[byte >> 3]
    return id_str


def parse_action_request(json_object: Any) -> tuple[str, Dict[str, Any]]:
    """
    Parses a Faroe action invocation parsed JSON object (not the body string). Since any realistic server will have
    support for parsing the JSON body into an object, we expect the object and not the JSON string.
    """

    if not isinstance(json_object, dict):
        raise ValueError("Invalid request object")

    if "action" not in json_object or not isinstance(json_object["action"], str):
        raise ValueError("Missing or invalid 'action' field")

    action = json_object["action"]

    if "arguments" not in json_object or not isinstance(json_object["arguments"], dict):
        raise ValueError("Missing or invalid 'arguments' field")

    action_arguments = json_object["arguments"]

    return action, action_arguments


def validate_str_argument(argument: str, arguments_json_object: Dict[str, Any]) -> str:
    if argument not in arguments_json_object or not isinstance(
        arguments_json_object[argument], str
    ):
        raise ValueError(f"Invalid or missing '{argument}' field")

    return arguments_json_object[argument]


def validate_base64_argument(
    argument: str, arguments_json_object: Dict[str, Any]
) -> bytes:
    if argument not in arguments_json_object or not isinstance(
        arguments_json_object[argument], str
    ):
        raise ValueError(f"Invalid or missing '{argument}' field")

    try:
        decoded_bytes = base64.b64decode(arguments_json_object[argument])
    except binascii.Error:
        raise ValueError(f"Invalid or missing '{argument}' field")

    return decoded_bytes


def validate_int_argument(argument: str, arguments_json_object: Dict[str, Any]) -> int:
    if argument not in arguments_json_object or not isinstance(
        arguments_json_object[argument], int
    ):
        raise ValueError(f"Invalid or missing '{argument}' field")

    return arguments_json_object[argument]


def validate_create_user_arguments(
    arguments_json_object: Dict[str, Any],
) -> tuple[str, bytes, str, bytes]:
    return (
        validate_str_argument("email_address", arguments_json_object),
        validate_base64_argument("password_hash", arguments_json_object),
        validate_str_argument("password_hash_algorithm_id", arguments_json_object),
        validate_base64_argument("password_salt", arguments_json_object),
    )


def validate_user_id_argument(arguments_json_object: Dict[str, Any]) -> str:
    return validate_str_argument("user_id", arguments_json_object)


def validate_email_address_argument(arguments_json_object: Dict[str, Any]) -> str:
    return validate_str_argument("email_address", arguments_json_object)


def validate_update_email_address_arguments(
    arguments_json_object: Dict[str, Any],
) -> tuple[str, str, int]:
    return (
        validate_str_argument("user_id", arguments_json_object),
        validate_str_argument("email_address", arguments_json_object),
        validate_int_argument("user_email_address_counter", arguments_json_object),
    )


def validate_update_password_hash_arguments(
    arguments_json_object: Dict[str, Any],
) -> tuple[str, bytes, str, bytes, int]:
    return (
        validate_str_argument("user_id", arguments_json_object),
        validate_base64_argument("password_hash", arguments_json_object),
        validate_str_argument("password_hash_algorithm_id", arguments_json_object),
        validate_base64_argument("password_salt", arguments_json_object),
        validate_int_argument("user_password_hash_counter", arguments_json_object),
    )


def validate_increment_sessions_counter_arguments(
    arguments_json_object: Dict[str, Any],
) -> tuple[str, int]:
    return (
        validate_str_argument("user_id", arguments_json_object),
        validate_int_argument("user_sessions_counter", arguments_json_object),
    )


def serialize_user_result(action_invocation_id: str, user: User) -> str:
    result_json_object = {
        "ok": True,
        "action_invocation_id": action_invocation_id,
        "values": {
            "user": {
                "id": user.id,
                "email_address": user.email_address,
                "password_hash": base64.b64encode(user.password_hash).decode(),
                "password_hash_algorithm_id": user.password_hash_algorithm_id,
                "password_salt": base64.b64encode(user.password_salt).decode(),
                "disabled": user.disabled,
                "display_name": user.display_name,
                "email_address_counter": user.email_address_counter,
                "password_hash_counter": user.password_hash_counter,
                "disabled_counter": user.disabled_counter,
                "sessions_counter": user.sessions_counter,
            }
        },
    }
    return json.dumps(result_json_object)


def serialize_empty_result(action_invocation_id: str) -> str:
    result_json_object = {
        "ok": True,
        "action_invocation_id": action_invocation_id,
        "values": {},
    }
    return json.dumps(result_json_object)


def serialize_error_result(action_invocation_id: str, error_code: str) -> str:
    result_json_object = {
        "ok": False,
        "action_invocation_id": action_invocation_id,
        "error_code": error_code,
    }
    result_json = json.dumps(result_json_object)
    return result_json


def process_request_gen(
    request_json_object: Any,
) -> Generator[Effect, Union[EffectResult, ActionError], ServerResult]:
    action, action_arguments = parse_action_request(request_json_object)
    action_invocation_id = generate_action_invocation_id()

    if action == "create_user":
        email_address, password_hash, password_hash_algorithm_id, password_salt = (
            validate_create_user_arguments(action_arguments)
        )
        effect = CreateUserEffect(
            action_invocation_id,
            email_address,
            password_hash,
            password_hash_algorithm_id,
            password_salt,
        )
        result = yield effect
        if not isinstance(result, User) and not isinstance(result, ActionError):
            raise TypeError(
                "Called `.send()` with incorrect type, expected User or ActionError!"
            )
    elif action == "get_user":
        user_id = validate_str_argument("user_id", action_arguments)
        effect = GetUserEffect(action_invocation_id, user_id)
        result = yield effect
        if not isinstance(result, User) and not isinstance(result, ActionError):
            raise TypeError(
                "Called `.send()` with incorrect type, expected User or ActionError!"
            )
    elif action == "get_user_by_email_address":
        email_address = validate_str_argument("email_address", action_arguments)
        effect = GetUserByEmailAddressEffect(action_invocation_id, email_address)
        result = yield effect
        if not isinstance(result, User) and not isinstance(result, ActionError):
            raise TypeError(
                "Called `.send()` with incorrect type, expected User or ActionError!"
            )
    elif action == "update_user_email_address":
        user_id, email_address, user_email_address_counter = (
            validate_update_email_address_arguments(action_arguments)
        )
        effect = UpdateUserEmailAddressEffect(
            action_invocation_id, user_id, email_address, user_email_address_counter
        )
        result = yield effect
        if result is not None and not isinstance(result, ActionError):
            raise TypeError(
                "Called `.send()` with incorrect type, expected None or ActionError!"
            )
    elif action == "update_user_password_hash":
        (
            user_id,
            password_hash,
            password_hash_algorithm_id,
            password_salt,
            user_password_hash_counter,
        ) = validate_update_password_hash_arguments(action_arguments)
        effect = UpdateUserPasswordHashEffect(
            action_invocation_id,
            user_id,
            password_hash,
            password_hash_algorithm_id,
            password_salt,
            user_password_hash_counter,
        )
        result = yield effect
        if result is not None and not isinstance(result, ActionError):
            raise TypeError(
                "Called `.send()` with incorrect type, expected None or ActionError!"
            )
    elif action == "increment_user_sessions_counter":
        user_id, user_sessions_counter = validate_increment_sessions_counter_arguments(
            action_arguments
        )
        effect = IncrementUserSessionsCounterEffect(
            action_invocation_id, user_id, user_sessions_counter
        )
        result = yield effect
        if result is not None and not isinstance(result, ActionError):
            raise TypeError(
                "Called `.send()` with incorrect type, expected None or ActionError!"
            )
    elif action == "delete_user":
        user_id = validate_str_argument("user_id", action_arguments)
        effect = DeleteUserEffect(action_invocation_id, user_id)
        result = yield effect
        if result is not None and not isinstance(result, ActionError):
            raise TypeError(
                "Called `.send()` with incorrect type, expected None or ActionError!"
            )
    else:
        return ServerResult("", f"Unknown action {action}")

    if result is None:
        return ServerResult(serialize_empty_result(action_invocation_id))
    elif isinstance(result, User):
        return ServerResult(serialize_user_result(action_invocation_id, result))
    else:
        return ServerResult(
            serialize_error_result(action_invocation_id, result.error_code),
            result.error_code,
        )


class SyncServer(Protocol):
    def execute_effect(self, effect: Effect) -> EffectResult: ...


class AsyncServer(Protocol):
    async def execute_effect(self, effect: Effect) -> EffectResult: ...


def handle_request_sync(request_json_object: Any, server: SyncServer) -> ServerResult:
    generator = process_request_gen(request_json_object)

    try:
        effect = next(generator)
        result = server.execute_effect(effect)
        generator.send(result)
    except StopIteration as e:
        assert isinstance(e.value, ServerResult)
        return e.value

    raise Exception("We do not expect any yields after send!")


async def handle_request_async(
    request_json_object: Any, server: AsyncServer
) -> ServerResult:
    generator = process_request_gen(request_json_object)

    try:
        effect = next(generator)
        result = await server.execute_effect(effect)
        generator.send(result)
    except StopIteration as e:
        assert isinstance(e.value, ServerResult)
        return e.value

    raise Exception("We do not expect any yields after send!")
