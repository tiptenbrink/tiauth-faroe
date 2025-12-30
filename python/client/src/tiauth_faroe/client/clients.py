from collections.abc import Awaitable, Generator
from typing import Callable, TypeVar, cast

from tiauth_faroe.client.logic import (
    ActionErrorResult,
    ActionParseResult,
    ActionResult,
    CompleteSigninActionSuccessResult,
    CompleteSignupActionSuccessResult,
    CreateSigninActionSuccessResult,
    CreateSignupActionSuccessResult,
    GetSessionActionSuccessResult,
    JSONDict,
    JSONValue,
    complete_signin,
    complete_signup,
    create_signin,
    create_signup,
    get_session,
    parse_action_invocation_response,
    send_signup_email_address_verification_code,
    set_signup_password,
    verify_signin_user_password,
    verify_signup_email_address_verification_code,
)


class AsyncClient:
    async def send_action_invocation_request(self, _body: JSONValue) -> JSONValue:
        raise NotImplementedError("Implement this function!")

    async def manage_action_invocation_request(
        self, action: str, arguments: JSONDict
    ) -> ActionParseResult:
        action_request: JSONDict = {"action": action, "arguments": arguments}

        response = await self.send_action_invocation_request(action_request)

        return parse_action_invocation_response(response)

    async def create_signup(
        self, email_address: str
    ) -> CreateSignupActionSuccessResult | ActionErrorResult:
        return await send_gen_async(
            create_signup(email_address),
            "create_signup",
            self.manage_action_invocation_request,
        )

    async def send_signup_email_address_verification_code(
        self, signup_token: str
    ) -> ActionResult:
        return await send_gen_async(
            send_signup_email_address_verification_code(signup_token),
            "send_signup_email_address_verification_code",
            self.manage_action_invocation_request,
        )

    async def verify_signup_email_address_verification_code(
        self, signup_token: str, email_address_verification_code: str
    ) -> ActionResult:
        return await send_gen_async(
            verify_signup_email_address_verification_code(
                signup_token, email_address_verification_code
            ),
            "verify_signup_email_address_verification_code",
            self.manage_action_invocation_request,
        )

    async def set_signup_password(
        self, signup_token: str, password: str
    ) -> ActionResult:
        return await send_gen_async(
            set_signup_password(signup_token, password),
            "set_signup_password",
            self.manage_action_invocation_request,
        )

    async def complete_signup(
        self, signup_token: str
    ) -> CompleteSignupActionSuccessResult | ActionErrorResult:
        return await send_gen_async(
            complete_signup(signup_token),
            "complete_signup",
            self.manage_action_invocation_request,
        )

    async def create_signin(
        self, user_email_address: str
    ) -> CreateSigninActionSuccessResult | ActionErrorResult:
        return await send_gen_async(
            create_signin(user_email_address),
            "create_signin",
            self.manage_action_invocation_request,
        )

    async def verify_signin_user_password(
        self, signin_token: str, password: str
    ) -> ActionResult:
        return await send_gen_async(
            verify_signin_user_password(signin_token, password),
            "verify_signin_user_password",
            self.manage_action_invocation_request,
        )

    async def complete_signin(
        self, signin_token: str
    ) -> CompleteSigninActionSuccessResult | ActionErrorResult:
        return await send_gen_async(
            complete_signin(signin_token),
            "complete_signin",
            self.manage_action_invocation_request,
        )

    async def get_session(
        self, session_token: str
    ) -> GetSessionActionSuccessResult | ActionErrorResult:
        return await send_gen_async(
            get_session(session_token),
            "get_session",
            self.manage_action_invocation_request,
        )


T = TypeVar("T")


async def send_gen_async(
    gen: Generator[JSONDict, ActionParseResult, T],
    action: str,
    send_request: Callable[[str, JSONDict], Awaitable[ActionParseResult]],
) -> T:
    args = next(gen)
    try:
        _ = gen.send(await send_request(action, args))
        raise Exception("unreachable!")
    except StopIteration as e:
        # This value should always be the third type parameter of generator
        return cast(T, e.value)


class SyncClient:
    def send_action_invocation_request(self, body: JSONValue) -> JSONValue:
        raise NotImplementedError("Implement this function!")

    def manage_action_invocation_request(
        self, action: str, arguments: JSONDict
    ) -> ActionParseResult:
        action_request: JSONDict = {"action": action, "arguments": arguments}

        response = self.send_action_invocation_request(action_request)

        return parse_action_invocation_response(response)

    def create_signup(
        self, email_address: str
    ) -> CreateSignupActionSuccessResult | ActionErrorResult:
        return send_gen_sync(
            create_signup(email_address),
            "create_signup",
            self.manage_action_invocation_request,
        )

    def send_signup_email_address_verification_code(
        self, signup_token: str
    ) -> ActionResult:
        return send_gen_sync(
            send_signup_email_address_verification_code(signup_token),
            "send_signup_email_address_verification_code",
            self.manage_action_invocation_request,
        )

    def verify_signup_email_address_verification_code(
        self, signup_token: str, email_address_verification_code: str
    ) -> ActionResult:
        return send_gen_sync(
            verify_signup_email_address_verification_code(
                signup_token, email_address_verification_code
            ),
            "verify_signup_email_address_verification_code",
            self.manage_action_invocation_request,
        )

    def set_signup_password(self, signup_token: str, password: str) -> ActionResult:
        return send_gen_sync(
            set_signup_password(signup_token, password),
            "set_signup_password",
            self.manage_action_invocation_request,
        )

    def complete_signup(
        self, signup_token: str
    ) -> CompleteSignupActionSuccessResult | ActionErrorResult:
        return send_gen_sync(
            complete_signup(signup_token),
            "complete_signup",
            self.manage_action_invocation_request,
        )

    def create_signin(
        self, user_email_address: str
    ) -> CreateSigninActionSuccessResult | ActionErrorResult:
        return send_gen_sync(
            create_signin(user_email_address),
            "create_signin",
            self.manage_action_invocation_request,
        )

    def verify_signin_user_password(
        self, signin_token: str, password: str
    ) -> ActionResult:
        return send_gen_sync(
            verify_signin_user_password(signin_token, password),
            "verify_signin_user_password",
            self.manage_action_invocation_request,
        )

    def complete_signin(
        self, signin_token: str
    ) -> CompleteSigninActionSuccessResult | ActionErrorResult:
        return send_gen_sync(
            complete_signin(signin_token),
            "complete_signin",
            self.manage_action_invocation_request,
        )

    def get_session(
        self, session_token: str
    ) -> GetSessionActionSuccessResult | ActionErrorResult:
        return send_gen_sync(
            get_session(session_token),
            "get_session",
            self.manage_action_invocation_request,
        )


def send_gen_sync(
    gen: Generator[JSONDict, ActionParseResult, T],
    action: str,
    send_request: Callable[[str, JSONDict], ActionParseResult],
) -> T:
    args = next(gen)
    try:
        _ = gen.send(send_request(action, args))
        raise Exception("unreachable!")
    except StopIteration as e:
        # This value should always be the third type parameter of generator
        return cast(T, e.value)
