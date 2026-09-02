# AGENTS.md — Mozia SSO Project Conventions

## Project Purpose

`mozia-sso` is the shared identity provider for the Mozia product family. It is based on Casdoor and exposes OAuth/OIDC login, authorization, token, userinfo, and identity administration capabilities.

Keep this repository focused on identity. It proves who a user is and issues a stable OIDC `sub`; it does not own Matrix sessions, reseller membership, customer ownership, pricing, wallet balances, usage, or settlement.

## Five-Repository Collaboration Map

| Repository | Responsibility | SSO relationship |
| --- | --- | --- |
| `mozia-sso` | Shared OIDC identity provider | Owns users, OIDC applications/clients, redirect URI allowlists, and identity claims |
| `mozia-mega` | Internal operations control plane | Uses its own OIDC client and server-side session; authorization is additionally restricted by Mega |
| `Matrix` | Ordinary customer product, wallet/API keys, and invite acceptance | Uses a separate OIDC client/session and sends only server-verified identity to its own backend integrations |
| `matrix-reseller` | Reseller-facing multi-tenant control plane | Uses a dedicated OIDC client/session; combines verified `sub` with direct Host through `mozia-api` to resolve tenant membership |
| `mozia-api` | AI gateway, billing, reseller and financial source of truth | Consumes stable subjects through trusted service contracts; it, not SSO, decides reseller membership and business authorization |

Runtime boundaries:

- `Mega / Matrix / matrix-reseller -> mozia-sso` are three independent OAuth/OIDC relying-party relationships.
- Do not share client secrets, application sessions, cookies, or local user IDs between those applications.
- The only cross-system user identity key is a server-verified OIDC `sub`; email, phone, username, Matrix-local ID, and Mega-local ID are not identity joins.
- A successful SSO login proves identity only. Mega controls internal-admin access, while `mozia-api` controls reseller membership and tenant permissions.
- `mozia-sso` must not call application databases or become the source of truth for reseller or billing data.

## Reseller Domain Changes

- Mega can edit a reseller Matrix Host, but that action updates the logical Host binding in `mozia-api`; Matrix resolves it dynamically.
- Every reseller domain used for login must have the exact `https://<host>/api/auth/callback` URI allowed on the dedicated `matrix-reseller` OIDC client before cutover.
- A complete Matrix domain switch requires DNS or a customer reverse proxy/TLS plus the `mozia-api` Host binding. Matrix custom domains return through its signed OAuth proxy, so its OIDC client keeps only the exact canonical callback and does not add one redirect per reseller domain.
- Keep Matrix, Mega, and reseller-management redirect URIs on their intended clients. Never solve a domain change by reusing another application's client secret or wildcarding redirects without an explicit security review.
- Reseller Logo is not SSO data. It is stored by `mozia-api` and rendered by `matrix-reseller`; no Logo copy belongs in this repository.

## Change Rules

- Preserve upstream Casdoor behavior unless a Mozia requirement explicitly needs a scoped change.
- Never commit production credentials, database passwords, signing keys, client secrets, access tokens, or unredacted user data.
- OIDC redirect changes are security-sensitive: use exact HTTPS origins in production, keep clients separated, and verify authorization-code, state, nonce, issuer, audience, and redirect checks.
- Preserve unrelated work in a dirty worktree. Run verification proportional to the touched code and report any checks that could not be run.
