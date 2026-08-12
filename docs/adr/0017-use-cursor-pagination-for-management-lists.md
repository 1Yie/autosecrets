# Use cursor pagination for management lists

Growing management lists will return one typed envelope containing `items`, an optional `next_cursor`, and an optional `total`, with stable search, structured filters, sorting, and Core-bounded limits. Because AutoSecrets has no released external management client, the existing unpaginated v1 array responses will migrate atomically with the Web client, OpenAPI contract, and tests before the first release instead of preserving two response shapes or introducing a premature v2.
