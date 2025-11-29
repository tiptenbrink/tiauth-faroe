export class UserClient {
  base: string;

  constructor(base: string) {
    this.base = base;
  }
  async sendRequest(endpoint: string, o: object): Promise<Response> {
    const response = await fetch(this.base + endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(o),
    });

    if (response.status !== 200) {
      throw new Error(`status: ${response.status}`);
    }

    return response;
  }

  async resetUsers() {
    await this.sendRequest("auth/clear_tables", {});
  }

  async prepareUser(email: string, names: string[]) {
    await this.sendRequest("auth/prepare_user", { email, names });
  }
}
