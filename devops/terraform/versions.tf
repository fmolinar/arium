terraform {
  required_version = ">= 1.6"

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

  # State stays local (terraform.tfstate, gitignored) while the stack is
  # created and destroyed by hand. It holds the MongoDB URI, so keep it off
  # shared disks; move it to an S3 backend before more people apply this.
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
