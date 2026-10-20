# terraform-aws-chatops

https://github.com/BarryPekerman/terraform-aws-chatops

A Terraform module for driving AWS infrastructure from a Telegram chat,
safely.

PROBLEM
-------
Spinning dev environments up and down is a chore that usually means opening a
laptop, assuming a role and running commands by hand. Wiring chat to
infrastructure is tempting but dangerous: a chat bot with broad cloud
credentials is an obvious way to delete the wrong thing.

BUILD
-----
- Telegram bot -> webhook -> GitHub Actions workflow, authenticated with
  GitHub OIDC, so no long-lived cloud keys live in the bot or in CI
- The Actions IAM role is scoped so it can only destroy resources tagged
  ChatOpsManaged=true -- the blast radius is fixed by a tag, not by trust
- Packaged as a reusable Terraform module so the whole pipeline is one
  `module` block in someone else's stack

RESULT
------
Tear down a tagged dev environment from your phone, with a credential path
that can't be replayed and a role that physically cannot touch anything
outside the ChatOpsManaged tag.

STACK: Terraform, AWS (IAM, OIDC), GitHub Actions, Telegram Bot API
