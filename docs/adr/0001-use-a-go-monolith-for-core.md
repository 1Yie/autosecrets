# Use a Go monolith for Core

Core will be a single Go service that owns the management API, Agent API, authentication, audit recording, and built Web assets. A typed single binary keeps deployment and operations simple, while the legacy Python CLI remains a migration reference and the Python Agent stays a separate runtime component.
