terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.7"
    }
  }

  # State lives in the bucket the bootstrap stack creates, so CI and a laptop
  # see the same resources. The bucket name contains the account ID, so it's
  # passed at init: -backend-config=bucket=<TF_STATE_BUCKET>
  # -backend-config=region=<region>. use_lockfile locks with an S3 object,
  # without a DynamoDB table. The state holds the MongoDB URI (the SSM
  # parameter's value); the bucket is private, encrypted and TLS-only.
  backend "s3" {
    key          = "collector/terraform.tfstate"
    use_lockfile = true
    encrypt      = true
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = var.project
      Component = "collector"
      ManagedBy = "terraform"
    }
  }
}
