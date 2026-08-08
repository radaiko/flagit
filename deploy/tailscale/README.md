# Tailscale serve config for the admin dashboard

`serve.json` is read by the `tailscale-admin` sidecar in `docker-compose.yml`
via `TS_SERVE_CONFIG`. It is the whole configuration of that node: an HTTPS
listener on 443 that reverse-proxies to the Flagit admin port, and nothing else.

```
https://flagit-admin.<tailnet>.ts.net/  →  http://flagit:3000
```

Notes on the shape of the file:

- `${TS_CERT_DOMAIN}` is substituted by the container with the node's own MagicDNS
  name, so the file does not have to hardcode the tailnet.
- `Proxy` targets the compose service name `flagit`, not `localhost` — the sidecar
  is a separate container. This only works while `TS_ACCEPT_DNS` stays `false`,
  which keeps Docker's embedded resolver in place.
- `AllowFunnel` is explicitly `false`. The dashboard must never leave the tailnet;
  Funnel would put it on the public internet.
- The directory, not the file, is mounted into the container. tailscaled watches
  it for changes, and a bind-mounted single file does not deliver those events.

The file is plain JSON — no comments, no trailing commas — because the container
parses it with a strict decoder.

## One-time tailnet setup

1. Enable HTTPS certificates for the tailnet (admin console → DNS → HTTPS
   Certificates). Without it there is no certificate for port 443 and the sidecar
   comes up unreachable.
2. Create an auth key (admin console → Settings → Keys). Tag it, for example
   `tag:flagit-admin`, so the node is not subject to key expiry; the tag needs an
   owner in the tailnet ACL policy.
3. Put that key in the deployment's `TS_AUTHKEY` secret. It is consumed on first
   start; the node identity then lives in the `tailscale_state` volume.
