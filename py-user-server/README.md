We use SQLite.

SQLite can work with threads, but they actually rather you don't. Instead, you can just make multiple connections. Because of the GIL, Python will be thread-safe. Any parallelism will be achieved by using multiple workers using e.g. gunicorn.

We try to limit the use of async, because most people don't know about it and we don't need it.
