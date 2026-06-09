terraform {
  required_providers {
    northflank = {
      source  = "vestmark-infra/northflank"
      version = "~> 0.1"
    }
  }
}

# Token may also be supplied via the NORTHFLANK_API_TOKEN environment variable.
provider "northflank" {
  api_token = var.northflank_api_token
}

variable "northflank_api_token" {
  description = "Northflank API token (Team Settings → API → Tokens)"
  type        = string
  sensitive   = true
}
