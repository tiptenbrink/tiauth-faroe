import json
from pathlib import Path
from http.server import HTTPServer, BaseHTTPRequestHandler
from tiauth_faroe.user_server import handle_request_sync
from data.db import init_sqlite_engine
from data.queries import SqliteSyncServer, clear_all_users
from data.model import metadata

class UserServerHTTPServer(HTTPServer):
    def __init__(self, server_address, handler_class, sync_server):
        super().__init__(server_address, handler_class)
        self.sync_server = sync_server

class JSONRequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        try:
            if self.path == '/auth/invoke_user_action':
                self.handle_invoke_user_action()
            elif self.path == '/auth/clear_tables':
                self.handle_clear_tables()
            elif self.path == '/auth/prepare_user':
                self.handle_prepare_user()
            else:
                self.send_error_response(404, "Not Found")

        except Exception as e:
            print(f"Error: {e}")
            self.send_error_response(500, "Internal Server Error")

    def handle_invoke_user_action(self):
        # Get content length
        content_length = int(self.headers.get('Content-Length', 0))

        # Read and parse JSON request
        raw_data = self.rfile.read(content_length)
        json_data = json.loads(raw_data.decode('utf-8'))

        # Process the request using handle_request_sync
        result = handle_request_sync(json_data, self.server.sync_server) # type: ignore
        response_data = result.response_json.encode('utf-8')

        # Send response
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(response_data)))
        self.end_headers()
        self.wfile.write(response_data)

    def handle_clear_tables(self):
        # Clear all users from the database
        clear_all_users(self.server.sync_server.engine) # type: ignore

        # Send empty 200 response
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', '2')
        self.end_headers()
        self.wfile.write(b'{}')

    def handle_prepare_user(self):
        # Send empty 200 response
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', '2')
        self.end_headers()
        self.wfile.write(b'{}')

    def send_error_response(self, status_code, message):
        """Send a JSON error response"""
        error_data = {"error": message, "status_code": status_code}
        json_response = json.dumps(error_data).encode('utf-8')

        self.send_response(status_code)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(json_response)))
        self.end_headers()
        self.wfile.write(json_response)

def run_server(host='localhost', port=8000, db_path=None):
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

if __name__ == "__main__":
    run_server()
