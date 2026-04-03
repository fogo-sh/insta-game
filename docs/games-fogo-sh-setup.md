# games.fogo.sh — post-deploy setup

After someone with Pulumi creds runs `pulumi up` in `pulumi/`, two manual steps are needed: validating the ACM certificate and adding the DNS record in Cloudflare.

## 1. Validate the ACM certificate

ACM needs proof you own `games.fogo.sh` before it will issue the cert. It does this via a CNAME record in DNS.

Get the record to add:

```sh
cd pulumi
pulumi stack output cert_validation_cname
```

This outputs something like:

```
[{"domain_name": "games.fogo.sh", "resource_record_name": "_abc123def456.games.fogo.sh.", "resource_record_type": "CNAME", "resource_record_value": "_xyz789.acm-validations.aws."}]
```

In Cloudflare DNS for `fogo.sh`, add:

| Type | Name | Value |
|------|------|-------|
| CNAME | `_abc123def456.games` | `_xyz789.acm-validations.aws.` |

> **Proxy must be OFF (grey cloud).** This is a validation record — Cloudflare proxying breaks it.

ACM checks for this record automatically. It usually validates within 2–5 minutes. `pulumi up` will wait for it — if it times out before the record propagates, just re-run `pulumi up` and it will continue once validated.

## 2. Add the games.fogo.sh CNAME

Once `pulumi up` completes successfully, get the CloudFront domain:

```sh
cd pulumi
pulumi stack output games_url
```

This outputs something like `d1a2b3c4d5e6f7.cloudfront.net`.

In Cloudflare DNS for `fogo.sh`, add:

| Type | Name | Value | Proxy |
|------|------|-------|-------|
| CNAME | `games` | `d1a2b3c4d5e6f7.cloudfront.net` | **OFF (grey cloud)** |

> **Proxy must be OFF.** CloudFront handles TLS using the ACM cert. If Cloudflare proxies the request, it will intercept TLS and break the cert. Grey cloud = DNS only.

DNS propagation is usually fast through Cloudflare. Once it propagates, `https://games.fogo.sh` should be live.

## Why proxy must be off

CloudFront terminates HTTPS using the ACM certificate issued for `games.fogo.sh`. If Cloudflare's proxy (orange cloud) is enabled, Cloudflare intercepts the connection and presents its own cert — CloudFront never sees the original hostname, and the ACM cert goes unused. Keeping it grey cloud means Cloudflare is DNS-only and the browser connects directly to CloudFront.

## Troubleshooting

**Site returns 403 from CloudFront** — the `Host` header isn't being forwarded correctly. The distribution uses the `AllViewerExceptHostHeader` origin request policy which rewrites Host to the Lambda URL domain. If you see this, confirm the distribution is fully deployed (status: `Deployed`, not `In Progress`).

**SSE log streaming doesn't work** — make sure you're accessing via `https://games.fogo.sh` and not the raw Lambda URL. The CloudFront distribution has compression disabled and caching disabled specifically to allow SSE to stream without buffering.

**Certificate stuck in `Pending validation`** — double-check the CNAME name and value are correct (ACM's output includes a trailing dot on the value, Cloudflare strips it automatically — that's fine). Make sure the record is proxy OFF.
