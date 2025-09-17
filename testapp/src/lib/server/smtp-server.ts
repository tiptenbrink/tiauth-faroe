import { SMTPServer, type SMTPServerSession } from "smtp-server";
import { simpleParser } from "mailparser";
import { createServer, Server } from "node:http";
import assert from "node:assert";

interface Address {
  name?: string;
  address: string;
}

interface Email {
  from: Address;
  to: Address;
  subject: string;
}

export interface SignupData {
  category: "signup";
  code: string;
}

export interface SigninDetectedData {
  category: "signinDetected";
}

export interface CategoryEmail<Data> {
  mail: Email;
  data: Data;
}

type CategoryDataMap = {
  signup: SignupData;
};

export type CategoryEmailByType<T extends keyof CategoryDataMap> =
  CategoryEmail<CategoryDataMap[T]>;

interface Emails {
  to: string;
  signupMails: CategoryEmail<SignupData>[];
  signinMails: CategoryEmail<SigninDetectedData>[];
}

function extractData(
  subject: string,
  text: string,
): SignupData | SigninDetectedData | undefined {
  if (subject === "Signup verification code") {
    const regex = /Your email address verification code is\s+(\w+)/i;
    const verificationCode = text.match(regex)?.[1];
    return {
      category: "signup",
      code: verificationCode!,
    };
  } else if (subject === "Sign-in detected") {
    return {
      category: "signinDetected",
    };
  }
  return undefined;
}

export class TestSMTPServer {
  server: SMTPServer | null;
  httpServer: Server | null;
  port: number;
  httpPort: number;
  map: Map<string, Emails>;

  constructor(port: number, httpPort: number = port + 1000) {
    this.port = port;
    this.httpPort = httpPort;
    this.map = new Map();
    this.server = null;
    this.httpServer = null;
  }

  private handleEmail(
    stream: NodeJS.ReadableStream,
    _session: SMTPServerSession,
    callback: (err?: Error) => void,
  ): void {
    let emailData = "";

    stream.on("data", (chunk) => {
      emailData += chunk.toString();
    });

    stream.on("end", async () => {
      try {
        const parsed = await simpleParser(emailData);
        const from = parsed.from!.value[0]!;
        const toAddr = parsed.to;
        assert(toAddr !== undefined);
        assert(!Array.isArray(toAddr));
        const to = toAddr.value[0]!;
        const subject = parsed.subject ?? "";
        const text = parsed.text ?? "";

        const mail = {
          to: {
            name: to.name,
            address: to.address!,
          },
          from: {
            name: from.name,
            address: from.address!,
          },
          subject: subject,
        };

        const toAddress = to.address!;
        let mails = this.map.get(toAddress);
        if (mails === undefined) {
          mails = {
            to: toAddress,
            signupMails: [],
            signinMails: [],
          };
          this.map.set(toAddress, mails);
        }

        const data = extractData(subject, text);
        if (data === undefined) {
          throw new Error("Could not extract data!");
        } else if (data.category === "signup") {
          mails.signupMails.push({
            mail,
            data,
          });
          console.log(JSON.stringify([...this.map.values()]));
        } else if (data.category === "signinDetected") {
          mails.signinMails.push({
            mail,
            data,
          });
        }

        callback();
      } catch (error) {
        console.error("Error parsing email:", error);
        callback(error as Error);
      }
    });
  }

  async start(): Promise<void> {
    this.server = new SMTPServer({
      secure: false,
      logger: false,
      // Shorter timeouts for testing
      socketTimeout: 10_000,
      closeTimeout: 500,
      authOptional: true,

      onData: (stream, session, callback) => {
        this.handleEmail(stream, session, callback);
      },
    });

    this.httpServer = createServer((req, res) => {
      const url = new URL(req.url!, `http://localhost:${this.httpPort}`);

      if (url.pathname === "/emails") {
        const email = url.searchParams.get("email");
        const category = url.searchParams.get("category");

        console.log(`request email=${email} cat=${category}`);

        if (!email || category !== "signup") {
          res.writeHead(400);
          res.end("Invalid parameters");
          return;
        }

        const emailData = this.map.get(email);
        if (!emailData || emailData.signupMails.length === 0) {
          res.writeHead(404);
          res.end("No emails found");
          return;
        }

        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(
          JSON.stringify(
            emailData.signupMails[emailData.signupMails.length - 1],
          ),
        );
      } else {
        console.log("unknown request path");
        res.writeHead(404);
        res.end("Not found");
      }
    });

    const promises: Promise<void>[] = [];

    promises.push(
      new Promise<void>((resolve, reject) => {
        this.server!.listen(this.port, (err?: Error) => {
          if (err) {
            reject(err);
          } else {
            console.log(`SMTP server listening on port ${this.port}`);
            resolve();
          }
        });
      }),
    );
    promises.push(
      new Promise<void>((resolve) => {
        this.httpServer!.listen(this.httpPort, () => {
          console.log(`HTTP server listening on port ${this.httpPort}`);
          resolve();
        });
      }),
    );

    await Promise.all(promises);
  }

  async stop(): Promise<void> {
    console.log("Starting server shutdown...");
    const promises: Promise<void>[] = [];
    // Close HTTP server
    promises.push(
      new Promise<void>((resolve) => {
        this.httpServer!.close((err) => {
          console.log("closed http!");
          if (err) console.error("HTTP server close error:", err);
          resolve();
        });
      }),
    );

    // Close SMTP server
    promises.push(
      new Promise<void>((resolve) => {
        this.server!.close(() => {
          console.log("closed smtp!");
          resolve();
        });
      }),
    );

    await Promise.all(promises);
    this.httpServer = null;
    this.server = null;
    console.log("SMTP and HTTP servers stopped");
  }
}
