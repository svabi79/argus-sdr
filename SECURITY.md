# Security Policy

Argus SDR is a single-maintainer project. Security reports are welcome.

## Reporting a vulnerability

Please **do not** open a public issue for a security problem. Instead use
GitHub's **private vulnerability reporting** for this repository
(the "Report a vulnerability" button under the **Security** tab), or email
the maintainer at `jan@svabi.ch`.

Include enough to reproduce: affected version/commit, steps, and impact.
You'll get an acknowledgement; fixes land via the normal PR + CI gate.

## Scope

Argus controls SDR receive hardware and exposes a local web/API server
(default `:8080`, bound to localhost). It is **receive-only** and intended for
local/trusted-network use; do not expose the API to the public internet without
your own authentication layer in front of it. Reports about the local server,
the demod/decode pipeline, dependency vulnerabilities, or the CI/release path
are all in scope.
