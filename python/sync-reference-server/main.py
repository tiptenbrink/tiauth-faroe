import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from tiauth_faroe.user_server import handle_request_sync

from sync_reference_server.data.db import init_sqlite_engine
from sync_reference_server.data.model import metadata
from sync_reference_server.data.queries import SqliteSyncServer, clear_all_users


# Default host/port matching what the Go tiauth server expects
DEFAULT_HOST = "127.0.0.2"
DEFAULT_PORT = 8079


class UserServerHTTPServer(ThreadingHTTPServer):
    sync_server: SqliteSyncServer

    def __init__(
        self,
        server_address: tuple[str, int],
        handler_class: type[BaseHTTPRequestHandler],
        sync_server: SqliteSyncServer,
    ):
        super().__init__(server_address, handler_class)
        self.sync_server = sync_server


class JSONRequestHandler(BaseHTTPRequestHandler):
    server: UserServerHTTPServer

    def do_POST(self):
        try:
            # /invoke is the endpoint the Go tiauth server calls
            if self.path == "/invoke" or self.path == "/auth/invoke_user_action":
                self.handle_invoke_user_action()
            elif self.path == "/auth/clear_tables":
                self.handle_clear_tables()
            elif self.path == "/auth/prepare_user":
                self.handle_prepare_user()
            else:
                self.send_error_response(404, "Not Found")

        except Exception as e:
            print(f"Error: {e}")
            self.send_error_response(500, "Internal Server Error")

    def do_GET(self):
        try:
            if self.path == "/health" or self.path == "/":
                self.handle_health()
            else:
                self.send_error_response(404, "Not Found")
        except Exception as e:
            print(f"Error: {e}")
            self.send_error_response(500, "Internal Server Error")

    def handle_health(self):
        """Health check endpoint."""
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", "15")
        self.end_headers()
        _ = self.wfile.write(b'{"status":"ok"}')

    def handle_invoke_user_action(self):
        content_length = int(self.headers.get("Content-Length", 0))

        raw_data = self.rfile.read(content_length)
        json_data = json.loads(raw_data.decode("utf-8"))

        result = handle_request_sync(json_data, self.server.sync_server)  # type: ignore
        response_data = result.response_json.encode("utf-8")

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response_data)))
        self.end_headers()
        _ = self.wfile.write(response_data)

    def handle_clear_tables(self):
        clear_all_users(self.server.sync_server.engine)  # type: ignore

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", "2")
        self.end_headers()
        _ = self.wfile.write(b"{}")

    def handle_prepare_user(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", "2")
        self.end_headers()
        _ = self.wfile.write(b"{}")

    def send_error_response(self, status_code: int, message: str):
        """Send a JSON error response"""
        error_data = {"error": message, "status_code": status_code}
        json_response = json.dumps(error_data).encode("utf-8")

        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(json_response)))
        self.end_headers()
        _ = self.wfile.write(json_response)


def run_server(host: str = DEFAULT_HOST, port: int = DEFAULT_PORT, db_path: Path | None = None):
    engine = init_sqlite_engine(Path(db_path) if db_path else None)

    metadata.create_all(engine)

    sync_server = SqliteSyncServer(engine)

    server = UserServerHTTPServer((host, port), JSONRequestHandler, sync_server)
    print(f"Server running on {host}:{port}")
    print("Press Ctrl+C to stop")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down server...")
        server.shutdown()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="tiauth-faroe user server")
    parser.add_argument(
        "--host",
        type=str,
        default=DEFAULT_HOST,
        help=f"Host to bind to (default: {DEFAULT_HOST})",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=DEFAULT_PORT,
        help=f"Port to bind to (default: {DEFAULT_PORT})",
    )
    parser.add_argument(
        "--db",
        type=str,
        default=None,
        help="Path to SQLite database file (default: in-memory)",
    )
    return parser.parse_args()


if __name__ == "__main__":
    args = parse_args()
    db_path = Path(args.db) if args.db else None
    run_server(args.host, args.port, db_path)
