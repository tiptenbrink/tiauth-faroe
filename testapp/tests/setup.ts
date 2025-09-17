import { TestSMTPServer } from "../src/lib/server/smtp-server.js";
import { UserClient } from "../src/lib/client.js";

let smtpServer = new TestSMTPServer(2525, 3525);
let userClient = new UserClient("http://localhost:8000/");

async function resetStores() {
  await Promise.all([
    userClient.resetUsers(),
    fetch("http://localhost:3777/reset"),
  ]);
}

export async function setup() {
  await Promise.all([smtpServer.start(), resetStores()]);
  console.log(`Setup complete!`);
}

export async function teardown() {
  await smtpServer.stop();
}
