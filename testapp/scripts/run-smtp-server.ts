import { TestSMTPServer } from "../src/lib/server/smtp-server.ts";

const SMTP_PORT = 2525;
const HTTP_PORT = 3525;

const server = new TestSMTPServer(SMTP_PORT, HTTP_PORT, true);

console.log("Starting SMTP test server...");
console.log(`SMTP will listen on port ${SMTP_PORT}`);
console.log(`HTTP API will listen on port ${HTTP_PORT}`);
console.log("\nTo query emails via HTTP:");
console.log(
  `  curl "http://localhost:${HTTP_PORT}/emails?email=user@example.com&category=signup"`,
);
console.log("\nAvailable categories:");
console.log("  - signup");
console.log("  - signinNotification");
console.log("  - emailUpdate");
console.log("  - emailUpdateNotification");
console.log("  - passwordReset");
console.log("  - passwordUpdateNotification");
console.log("\nPress Ctrl+C to stop the server\n");

await server.start();

// Graceful shutdown
process.on("SIGINT", async () => {
  console.log("\nShutting down...");
  await server.stop();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  console.log("\nShutting down...");
  await server.stop();
  process.exit(0);
});
