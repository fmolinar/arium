terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # This stack creates the bucket the collector stack keeps its state in, so
  # its own state stays local (terraform.tfstate here, gitignored). It holds no
  # secrets. If it's lost, `terraform import` the resources back.
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = var.project
      Component = "bootstrap"
      ManagedBy = "terraform"
    }
  }
}
