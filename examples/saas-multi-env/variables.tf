# The three things you must change to run this against your own account.

variable "repository" {
  description = "Source repository for the application, as owner/name."
  type        = string
  default     = "acme-co/invoicing"
}

variable "region" {
  description = "Region for every resource in this example. List the valid values with the laravel_cloud_regions data source."
  type        = string
  default     = "us-east-2"
}

variable "domain" {
  description = "Domain served by the production environment. You must control its DNS for the domain to verify."
  type        = string
  default     = "invoicing.example.com"
}
