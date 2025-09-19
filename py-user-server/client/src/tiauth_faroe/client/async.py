from typing import TypeVar, cast, Callable
from collections.abc import Generator, Awaitable
from tiauth_faroe.client import ActionErrorResult, ActionResult, GetSessionActionSuccessResult, JSONValue, get_session, JSONDict, parse_action_invocation_response

class Client:
    async def send_action_invocation_request(self, _body: JSONValue) -> JSONValue:
        raise NotImplementedError("Implement this function!")

    async def manage_action_invocation_request(self, action: str, arguments: JSONDict) -> ActionResult:
        action_request: JSONDict = {
            "action": action,
            "arguments": arguments
        }

        response = await self.send_action_invocation_request(action_request)

        return parse_action_invocation_response(response)


    async def get_session(self, session_token: str) -> GetSessionActionSuccessResult | ActionErrorResult:
        return await send_gen(get_session(session_token), 'get_session', self.manage_action_invocation_request)


T = TypeVar('T')


async def send_gen(gen: Generator[JSONDict, ActionResult, T], action: str, send_request: Callable[[str, JSONDict], Awaitable[ActionResult]]) -> T:
    args = next(gen)
    try:
        _ = gen.send(await send_request(action, args))
        raise Exception("unreachable!")
    except StopIteration as e:
        # This value should always be the third type parameter of generator
        return cast(T, e.value)
