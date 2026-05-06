# Context

## Terms

### Foam app

A client application that uses `foam-proxy` to exchange or refresh Twitch tokens and to build redirect URIs. Foam apps are identified by the `app` query parameter, such as `foam-app` or `foam-menubar`.

### Foam app capability collection

The runtime collection of configured Foam apps loaded from environment variables. It preserves the first stable occurrence order from `PROXY_APPS`, supports lookup by app key, and may return warnings for duplicate app entries that were ignored.
