# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in glsync, please **do not** open a public GitHub issue.

Instead, email: **saddamh2403@gmail.com**

Include:
- A description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested fix (optional)

You will receive a response within 72 hours. Once the issue is confirmed and a fix is ready, a security advisory will be published alongside the patched release.

## Scope

- Webhook token validation bypass
- SQL injection in job/event queries
- Authentication bypass on admin endpoints
- Credential exposure in logs or error messages

## Out of Scope

- Vulnerabilities in Jira or GitLab themselves
- Denial of service via webhook flood (rate limiting is left to the reverse proxy)
