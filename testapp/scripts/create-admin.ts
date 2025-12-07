import { Client, type ActionInvocationEndpointClient } from "@faroe/client";
import type { CategoryEmailByType } from "../src/lib/server/smtp-server.ts";

const FAROE_ENDPOINT = "http://localhost:3777/";
const USER_SERVER_ENDPOINT = "http://localhost:8000/private/";
const SMTP_HTTP_PORT = 3525;
const ADMIN_EMAIL = "admin@example.com";
const ADMIN_FIRSTNAME = "Admin";
const ADMIN_LASTNAME = "User";

class EndpointClient implements ActionInvocationEndpointClient {
  public async sendActionInvocationEndpointRequest(body: string) {
    const response = await fetch(FAROE_ENDPOINT, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: body,
    });
    if (response.status !== 200) {
      console.error(
        `Failed response:\n${await response.text()}\nfor body:\n${body}`,
      );
      throw new Error(`Unknown status ${response.status}`);
    }
    return await response.text();
  }
}

async function fetchMailData<T extends "signup">(
  email: string,
  category: T,
): Promise<CategoryEmailByType<T>> {
  const smtpHttpResponse = await fetch(
    `http://localhost:${SMTP_HTTP_PORT}/emails?email=${encodeURIComponent(email)}&category=${encodeURIComponent(category)}`,
  );
  if (smtpHttpResponse.status !== 200) {
    throw new Error(
      `Failed to fetch email: ${smtpHttpResponse.status} - ${await smtpHttpResponse.text()}`,
    );
  }
  const emailData = await smtpHttpResponse.json();
  return emailData as CategoryEmailByType<T>;
}

function generateSecurePassword(): string {
  const length = 20;
  const charset =
    "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*";
  const array = new Uint8Array(length);
  crypto.getRandomValues(array);
  let password = "";
  for (let i = 0; i < length; i++) {
    password += charset[array[i]! % charset.length];
  }
  return password;
}

async function addAdminPermission(userId: string) {
  const response = await fetch(
    `${USER_SERVER_ENDPOINT}add_admin_permission/`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ user_id: userId }),
    },
  );

  if (response.status !== 200) {
    throw new Error(
      `Failed to add admin permission: ${response.status} - ${await response.text()}`,
    );
  }

  return await response.text();
}

async function main() {
  console.log("=== Creating Admin User ===");
  console.log("Note: This assumes the backend is running in test mode\n");

  const client = new Client(new EndpointClient());

  // Generate secure password
  const password = generateSecurePassword();

  try {
    // Step 1: Prepare user in the user store
    console.log(`\nPreparing user in database...`);
    const prepareResponse = await fetch(
      `${USER_SERVER_ENDPOINT}prepare_user`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          email: ADMIN_EMAIL,
          names: [ADMIN_FIRSTNAME, ADMIN_LASTNAME],
        }),
      },
    );
    if (prepareResponse.status !== 200) {
      throw new Error(
        `Failed to prepare user: ${prepareResponse.status} - ${await prepareResponse.text()}`,
      );
    }
    console.log(`✓ User prepared: ${ADMIN_EMAIL}`);

    // Step 2: Create signup
    console.log("\nCreating signup...");
    const signupResult = await client.createSignup(ADMIN_EMAIL);
    if (!signupResult.ok) {
      throw new Error(`Signup failed: ${JSON.stringify(signupResult)}`);
    }
    const signupToken = signupResult.signupToken;
    console.log("✓ Signup created");

    // Step 3: Fetch verification code from SMTP server
    console.log("\nFetching verification code from SMTP server...");
    const emailData = await fetchMailData(ADMIN_EMAIL, "signup");
    const verificationCode = emailData.data.code;
    console.log(`✓ Verification code received: ${verificationCode}`);

    // Step 4: Verify email address
    console.log("\nVerifying email address...");
    const verifyResult = await client.verifySignupEmailAddressVerificationCode(
      signupToken,
      verificationCode,
    );
    if (!verifyResult.ok) {
      throw new Error(`Verification failed: ${JSON.stringify(verifyResult)}`);
    }
    console.log("✓ Email verified");

    // Step 5: Set password
    console.log("\nSetting password...");
    const passwordResult = await client.setSignupPassword(
      signupToken,
      password,
    );
    if (!passwordResult.ok) {
      throw new Error(`Set password failed: ${JSON.stringify(passwordResult)}`);
    }
    console.log("✓ Password set");

    // Step 6: Complete signup
    console.log("\nCompleting signup...");
    const completeResult = await client.completeSignup(signupToken);
    if (!completeResult.ok) {
      throw new Error(
        `Complete signup failed: ${JSON.stringify(completeResult)}`,
      );
    }
    const userId = completeResult.session.userId;
    console.log(`✓ Signup complete - User ID: ${userId}`);

    // Step 7: Add admin permission
    console.log("\nAdding admin permission...");
    await addAdminPermission(userId);
    console.log("✓ Admin permission added");

    // Success!
    console.log("\n" + "=".repeat(50));
    console.log("✓ Admin user created successfully!");
    console.log("=".repeat(50));
    console.log(`\nEmail:    ${ADMIN_EMAIL}`);
    console.log(`Password: ${password}`);
    console.log(`User ID:  ${userId}`);
    console.log(
      "\n⚠️  Save this password securely - it won't be shown again!\n",
    );
  } catch (error) {
    console.error("\n✗ Failed to create admin user:");
    console.error(error);
    process.exit(1);
  }
}

main();
