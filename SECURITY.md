# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in pgpipe, please report it responsibly using **GitHub Security Advisories**:

1. Go to [https://github.com/RobertoGongora/pgpipe/security/advisories/new](https://github.com/RobertoGongora/pgpipe/security/advisories/new)
2. Fill in the advisory details
3. Submit the report

**Please do not open a public issue for security vulnerabilities.**

## What to expect

- We will acknowledge your report within 7 days.
- We will provide an estimated timeline for a fix.
- We will notify you when the vulnerability is fixed.
- We will credit you in the release notes (unless you prefer to remain anonymous).

## Scope

This policy applies to the pgpipe codebase and its official releases. It does not cover:

- Vulnerabilities in upstream dependencies (report those to the respective projects)
- Vulnerabilities in MySQL or PostgreSQL themselves
- Issues with user-provided configuration or credentials

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | Best effort |

## Security best practices for pgpipe users

- Store database passwords in environment variables or `.env` files, not in YAML config files.
- Restrict file permissions on `.pgpipe/config.yaml` and `.env` (they may contain credentials).
- Use `PGSQL_SSLMODE=require` when connecting to hosted PostgreSQL providers over the internet.
