import {
  Client,
  type ActionInvocationEndpointClient,
  type Session,
} from "@faroe/client";
import {
  type CategoryEmailByType,
  type CategoryDataMap,
} from "../src/lib/server/smtp-server.ts";
import { test, assert, describe } from "vitest";
import { UserClient } from "../src/lib/client.ts";
import { faker } from "@faker-js/faker";

const endpoint = "http://localhost:3777/";

class EndpointClient implements ActionInvocationEndpointClient {
  public async sendActionInvocationEndpointRequest(body: string) {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: body,
    });
    if (response.status !== 200) {
      console.error(
        `failed response=\n${await response.text()} for body\n${body}`,
      );
      throw new Error(`Unknown status ${response.status}`);
    }
    const resultJSON = await response.text();
    return resultJSON;
  }
}

const actionInvocationEndpointClient = new EndpointClient();

const client = new Client(actionInvocationEndpointClient);

const testPass = "N9u1%e0!Bc*2*wQ$";

const userClient = new UserClient("http://localhost:8000/");

async function fetchMailData<T extends keyof CategoryDataMap>(
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

const testEmail = "someuser@gmail.com";

// Tests are run in sequence (https://vitest.dev/guide/parallelism.html#file-parallelism)

test("signup", async () => {
  await userClient.prepareUser(testEmail, ["Some", "User"]);

  const r = await client.createSignup(testEmail);
  assert(r.ok, JSON.stringify(r));

  const token = r.signupToken;

  const emailData = await fetchMailData(testEmail, "signup");
  const code = emailData.data.code;

  const r2 = await client.verifySignupEmailAddressVerificationCode(token, code);
  assert(r2.ok);

  const r3 = await client.setSignupPassword(token, testPass);
  assert(r3.ok);

  const rend = await client.completeSignup(token);
  assert(rend.ok);
});

test("signin", async () => {
  const r = await client.createSignin(testEmail);
  assert(r.ok, JSON.stringify(r));

  const r2 = await client.verifySigninUserPassword(r.signinToken, testPass);
  assert(r2.ok);

  const r3 = await client.completeSignin(r.signinToken);
  assert(r3.ok);
  console.log(`token for ${r3.session.userId}: ${r3.sessionToken}`);
});

interface User {
  firstname: string;
  lastname: string;
  email: string;
}

export const userTest = test.extend<{
  user: User & { sessionToken: string; password: string };
}>({
  user: async ({}, use) => {
    const firstname = faker.person.firstName();
    const lastname = faker.person.lastName();
    const email = faker.internet.email({
      firstName: firstname,
      lastName: lastname,
    });
    const password = faker.internet.password();
    await userClient.prepareUser(email, [firstname, lastname]);

    const r = await client.createSignup(email);
    assert(r.ok);

    const token = r.signupToken;

    const emailData = await fetchMailData(email, "signup");
    const code = emailData.data.code;

    const r2 = await client.verifySignupEmailAddressVerificationCode(
      token,
      code,
    );
    assert(r2.ok);

    const r3 = await client.setSignupPassword(token, password);
    assert(r3.ok);

    const rend = await client.completeSignup(token);
    assert(rend.ok);

    await use({
      firstname,
      lastname,
      email: email,
      sessionToken: rend.sessionToken,
      password,
    });
  },
});

async function signinUser(
  email: string,
  password: string,
): Promise<Session & { token: string }> {
  const r = await client.createSignin(email);
  assert(r.ok);

  const r2 = await client.verifySigninUserPassword(r.signinToken, password);
  assert(r2.ok);

  const r3 = await client.completeSignin(r.signinToken);
  assert(r3.ok);

  return { ...r3.session, token: r3.sessionToken };
}

describe.concurrent("suite", () => {
  userTest("session", async ({ user }) => {
    const r = await client.getSession(user.sessionToken);
    assert(r.ok);

    const signinSession = await signinUser(user.email, user.password);

    const r2 = await client.deleteSession(user.sessionToken);
    assert(r2.ok);

    const r3 = await client.getSession(user.sessionToken);
    assert(!r3.ok);

    const signinSession2 = await signinUser(user.email, user.password);
    const r4 = await client.deleteAllSessions(signinSession.token);
    assert(r4.ok, JSON.stringify(r4));

    const [r5, r6] = await Promise.all([
      client.getSession(signinSession.token),
      client.getSession(signinSession2.token),
    ]);
    assert(!r5.ok);
    assert(!r6.ok);
  });

  userTest("email update", async ({ user }) => {
    const newEmail = faker.internet.email({
      firstName: user.firstname,
      lastName: user.lastname,
      provider: "update" + faker.number.int({ min: 0, max: 100000 }) + ".com",
    });
    const r = await client.createUserEmailAddressUpdate(
      user.sessionToken,
      newEmail,
    );
    assert(r.ok);

    const updateToken = r.userEmailAddressUpdateToken;

    const emailData = await fetchMailData(newEmail, "emailUpdate");
    const code = emailData.data.code;

    const r2 =
      await client.verifyUserEmailAddressUpdateEmailAddressVerificationCode(
        user.sessionToken,
        updateToken,
        code,
      );
    assert(r2.ok);

    const r3 = await client.verifyUserEmailAddressUpdateUserPassword(
      user.sessionToken,
      updateToken,
      user.password,
    );
    assert(r3.ok);

    const r4 = await client.completeUserEmailAddressUpdate(
      user.sessionToken,
      updateToken,
    );
    assert(r4.ok);

    // TODO check if result succeeded?
  });

  userTest("password update", async ({ user }) => {
    const r = await client.createUserPasswordUpdate(user.sessionToken);
    assert(r.ok);
    const updateToken = r.userPasswordUpdateToken;

    const r2 = await client.verifyUserPasswordUpdateUserPassword(
      user.sessionToken,
      updateToken,
      user.password,
    );
    assert(r2.ok);

    const newPassword = faker.internet.password();
    const r3 = await client.setUserPasswordUpdateNewPassword(
      user.sessionToken,
      updateToken,
      newPassword,
    );
    assert(r3.ok);

    const r4 = await client.completeUserPasswordUpdate(
      user.sessionToken,
      updateToken,
    );
    assert(r4.ok);

    await signinUser(user.email, newPassword);
  });

  userTest("delete user", async ({ user }) => {
    const r = await client.createUserDeletion(user.sessionToken);
    assert(r.ok);
    const deleteToken = r.userDeletionToken;

    const r2 = await client.verifyUserDeletionUserPassword(
      user.sessionToken,
      deleteToken,
      user.password,
    );
    assert(r2.ok);

    const r3 = await client.completeUserDeletion(
      user.sessionToken,
      deleteToken,
    );
    assert(r3.ok);
  });

  userTest("reset password", async ({ user }) => {
    const r = await client.createUserPasswordReset(user.email);
    assert(r.ok);
    const resetToken = r.userPasswordResetToken;

    const r2 = await fetchMailData(user.email, "passwordReset");
    const tempPassword = r2.data.tempPassword;
    const r3 = await client.verifyUserPasswordResetTemporaryPassword(
      resetToken,
      tempPassword,
    );
    assert(r3.ok, JSON.stringify(r3));

    const newPassword = faker.internet.password();
    const r4 = await client.setUserPasswordResetNewPassword(
      resetToken,
      newPassword,
    );
    assert(r4.ok);

    const r5 = await client.completeUserPasswordReset(resetToken);
    assert(r5.ok);
  });
});
