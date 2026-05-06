# Security Policy

## Prevention

We have a few preventive measures in place to detect security vulnerabilities:

- [Renovate](https://docs.renovatebot.com/) helps us keep our dependencies up-to-date to patch vulnerabilities as soon as possible by creating awareness and automated PRs.
- [Snyk](https://snyk.io/) helps us ship secure container images:
  - Images are scanned in every pull request (PR) to detect new vulnerabilities.
  - Published images on GitHub Container Registry are monitored to detect new vulnerabilities so we can ship patches
- [Semgrep](https://semgrep.dev/) helps us with identifying vulnerabilities in our code to raise awareness.
- [FOSSA](https://fossa.com) scans our dependencies for license compliance and known vulnerabilities.
- [GitHub's security features](https://github.com/features/security) are constantly monitoring our repo and dependencies:
  - All pull requests (PRs) use CodeQL to scan our source code for vulnerabilities
  - Dependabot will automatically identify vulnerabilities based on the GitHub Advisory Database and open PRs with patches
  - Automated [secret scanning](https://docs.github.com/en/enterprise-cloud@latest/code-security/concepts/secret-security/about-secret-scanning) and alerts
- Container images are signed with [cosign](https://github.com/sigstore/cosign) using keyless OIDC via GitHub Actions
- All GitHub Actions are pinned to full commit SHAs to prevent supply chain attacks

## Disclosures

We strive to ship secure software, but we need the community to help us find security breaches.

In case of a confirmed breach, reporters will get full credit and can be kept in the loop, if preferred.

### Private Disclosure Processes

We ask that all suspected vulnerabilities be privately and responsibly disclosed via one of:

- [GitHub Private Vulnerability Reporting](https://github.com/kedacore/http-add-on/security/advisories/new) (preferred)
- Email to [KEDA maintainers](mailto:cncf-keda-maintainers@lists.cncf.io)

### Public Disclosure Processes

If you know of a publicly disclosed security vulnerability please IMMEDIATELY email the [KEDA maintainers](mailto:cncf-keda-maintainers@lists.cncf.io) to inform about the vulnerability so they may start the patch, release, and communication process.

## Communication

[GitHub Security Advisory](https://github.com/kedacore/http-add-on/security/advisories) will be used to communicate during the process of identifying, fixing, and shipping the mitigation of the vulnerability.

The advisory will only be made public when the patched version is released to inform the community of the breach and its potential security impact.

## Security Scope

Vulnerabilities in the HTTP Add-on components (interceptor, operator, scaler) are in scope. This includes authentication/authorization bypasses, request smuggling, denial of service, and privilege escalation within the components.

Misconfiguration of upstream Kubernetes RBAC, network policies, or KEDA core is out of scope.
