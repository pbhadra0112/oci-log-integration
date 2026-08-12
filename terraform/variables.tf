variable "tenancy_ocid" {
  type        = string
  description = "OCI tenant OCID, more details can be found at https://docs.cloud.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five"
}

variable "compartment_ocid" {
  type        = string
  description = "The OCID of the compartment where resources will be created. Do not modify."
}

variable "newrelic_logging_identifier" {
  type        = string
  description = "A unique label or name identifier for all resources in this deployment. Leave it blank if not needed."
  default     = "logs"
}

variable "region" {
  type        = string
  description = "The name of the OCI region where these resources will be deployed."
}

variable "new_relic_region" {
  type        = string
  default     = "US"
  description = "New Relic Region. US, EU, or JP"
}

variable "newrelic_account_id" {
  type        = string
  sensitive   = true
  description = "The New Relic account ID for sending metrics to New Relic endpoints"
}

variable "create_vcn" {
  type        = bool
  default     = true
  description = "Variable to create virtual network for the setup. True by default"
}

variable "function_subnet_id" {
  type        = string
  default     = ""
  description = "The OCID of the subnet to be used for the function app. If create_vcn is set to true, that will take precedence"
}

variable "payload_link" {
  type        = string
  description = "The link to the payload for the connector hubs."
  default     = ""
}

variable "debug_enabled" {
  type        = string
  default     = "FALSE"
  description = "Enable debug mode."
}

variable "image_version" {
  type        = string
  description = "The version of the Docker image for the New Relic function for the region."
  default     = "latest"
}

variable "metrics_tier" {
  type        = string
  default     = "none"
  description = "Tier of forwarder.* custom metrics the function emits about itself (in addition to New Relic's own ingested logs): none (no custom metrics) or basic (core health metrics: invocations, records received/delivered/dropped, delivery duration, pipeline lag). An advanced tier with deeper root-cause/tuning metrics is planned but not yet implemented. Custom metrics are billed by New Relic on ingest; none is the default so this is opt-in."

  validation {
    condition     = contains(["none", "basic"], var.metrics_tier)
    error_message = "metrics_tier must be one of: none, basic."
  }
}