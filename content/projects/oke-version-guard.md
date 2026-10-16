# oke-version-guard

https://github.com/BarryPekerman/oke-version-guard

Keeps a free-tier Oracle OKE cluster on a supported Kubernetes version,
because an unsupported cluster starts costing $0.10/hour.

- Weekly GitHub Actions check, authenticated with a least-privilege OCI API key
- Opens an issue when an upgrade is due; can upgrade automatically
- Upgrades one minor version at a time and cycles nodes one by one, with
  no surge, so the cluster never leaves the free tier
- Backs up attached block volumes first, and fails closed on any unhealthy state
