# Security Policy

## Supported versions

Before the first tagged release, `master` is the supported version. After that, the latest release is supported. Older releases do not receive security fixes.

## Reporting a vulnerability

Do not open a public issue for a security vulnerability.

Report it through GitHub private vulnerability reporting:

[https://github.com/balinomad/prompt-backlog/security/advisories/new](https://github.com/balinomad/prompt-backlog/security/advisories/new)

Include:

- The affected version or commit.
- The operating system and Go version.
- A minimal reproduction.
- The impact.
- Any suggested fix.

Remove credentials, personal data, prompt text, and private paths from the report.

## Scope

`prompt-backlog` is a local terminal application. Security-sensitive areas include:

- Atomic JSON writes and temporary file handling.
- File and directory permissions for the data file and its parent directory.
- Path handling for the `--data` / `-d` flag.
- Terminal input parsing and rendering.
- Clipboard integration.
- Handling of malformed or corrupted data files.

The application makes no network requests and stores no credentials. Vulnerabilities in LLM vendor services are out of scope.

## Response

Reports are reviewed on a best-effort basis. There is no guaranteed response time or fix deadline.
