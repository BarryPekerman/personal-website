# terraform-aws-chatops

https://github.com/BarryPekerman/terraform-aws-chatops

A Terraform module for managing AWS infrastructure from Telegram.

- Telegram bot -> webhook -> GitHub Actions workflow, authenticated with GitHub OIDC
- The Actions IAM role can only destroy resources tagged ChatOpsManaged=true
- Example: tear down a dev environment from your phone
