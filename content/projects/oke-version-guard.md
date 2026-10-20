# oke-version-guard

https://github.com/BarryPekerman/oke-version-guard

Keeps a free-tier Oracle OKE cluster on a supported Kubernetes version,
automatically, without ever leaving the free tier.

PROBLEM
-------
Oracle's free-tier OKE is free only while the cluster runs a supported
Kubernetes version. Let it fall out of support and it starts costing
$0.10/hour. Upgrades are easy to forget, and a careless upgrade can surge
nodes or leave volumes unprotected -- either of which can break the free tier
or lose data.

BUILD
-----
- A weekly GitHub Actions check, authenticated with a least-privilege OCI API
  key, that compares the cluster against Oracle's supported versions
- Opens a tracking issue when an upgrade is due, and can apply it automatically
- Upgrades one minor version at a time and cycles nodes one by one with no
  surge, so the node count never exceeds the free-tier limit
- Backs up attached block volumes before touching anything, and fails closed
  on any unhealthy state rather than pressing on

RESULT
------
A cluster that stays current and in-support with zero manual effort and zero
cost. No surprise $0.10/hour bills, no data-at-risk upgrades.

STACK: Oracle Cloud (OKE), Kubernetes, GitHub Actions, OCI API (least-privilege)
