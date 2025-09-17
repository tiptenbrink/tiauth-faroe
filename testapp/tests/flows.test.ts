import * as faroe_client from "@faroe/client";
import { type CategoryEmailByType } from "../src/lib/server/smtp-server.ts";
import { test, assert } from "vitest";
import { UserClient } from "../src/lib/client.ts";

const endpoint = "http://localhost:3777/";

class ActionInvocationEndpointClient
  implements faroe_client.ActionInvocationEndpointClient
{
  public async sendActionInvocationEndpointRequest(body: string) {
    // Handle authentication, etc
    const response = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: body,
    });
    if (response.status !== 200) {
      throw new Error(`Unknown status ${response.status}`);
    }
    const resultJSON = await response.text();
    return resultJSON;
  }
}

const actionInvocationEndpointClient = new ActionInvocationEndpointClient();

const client = new faroe_client.Client(actionInvocationEndpointClient);

const testPass = "N9u1%e0!Bc*2*wQ$";

const userClient = new UserClient("http://localhost:8000/");

async function fetchVerificationCode<T extends "signup">(
  email: string,
  category: T,
): Promise<CategoryEmailByType<T>> {
  const smtpHttpResponse = await fetch(
    `http://localhost:3525/emails?email=${encodeURIComponent(email)}&category=${encodeURIComponent(category)}`,
  );
  if (smtpHttpResponse.status !== 200) {
    throw new Error(
      `Failed to fetch verification code: ${smtpHttpResponse.status}`,
    );
  }
  const emailData = await smtpHttpResponse.json();
  return emailData as CategoryEmailByType<T>;
}

test("signup", async () => {
  await userClient.resetUsers();
  await userClient.prepareUser("someuser@gmail.com", ["Some", "User"]);

  const r = await client.createSignup("someuser@gmail.com");
  assert(r.ok);

  const token = r.signupToken;

  const emailData = await fetchVerificationCode("someuser@gmail.com", "signup");
  const code = emailData.data.code;

  const r2 = await client.verifySignupEmailAddressVerificationCode(token, code);
  assert(r2.ok);

  const r3 = await client.setSignupPassword(token, testPass);
  assert(r3.ok);

  const rend = await client.completeSignup(token);
  assert(rend.ok);
});

// async function testSignin() {
//   const r = await client.createSignin("someuser@gmail.com");
//   if (!r.ok) {
//     throw new Error(r.errorCode);
//   }
//   const r2 = await client.verifySigninUserPassword(r.signinToken, testPass);
//   if (!r2.ok) {
//     throw new Error(r2.errorCode);
//   }
//   const r3 = await client.completeSignin(r.signinToken);
//   console.log(JSON.stringify(r3));
// }
