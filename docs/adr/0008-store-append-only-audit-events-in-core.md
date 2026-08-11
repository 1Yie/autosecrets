# Store append-only Audit Events in Core

Core will append an Audit Event for every security-relevant human and Agent action, including failed outcomes, while excluding Secret values. The application will expose audit queries but no mutation or deletion API, giving self-hosted operators useful evidence without requiring an external logging platform.
