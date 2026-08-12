terraform {
  required_version = ">= 1.2.0"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "7.12.0"
    }
  }
}

# Variables
provider "oci" {
  alias        = "home_provider"
  tenancy_ocid = var.tenancy_ocid
  region       = var.region
}

# Resource for the logging function application
resource "oci_functions_application" "logging_function_app" {
  compartment_id = local.compartment_ocid
  config = {
    "VAULT_REGION"           = var.region
    "DEBUG_ENABLED"          = var.debug_enabled
    "SECRET_OCID"            = local.ingest_key_secret_ocid
    "CLIENT_TTL"             = local.client_ttl
    "NEW_RELIC_REGION"       = var.new_relic_region
    "FORWARDER_METRICS_TIER" = var.metrics_tier
  }
  defined_tags               = {}
  display_name               = local.function_app_name
  freeform_tags              = local.freeform_tags
  network_security_group_ids = []
  shape                      = local.function_app_shape
  subnet_ids = [
    data.oci_core_subnet.input_subnet.id,
  ]
}

#Resource for creating a log group for function logs
resource "oci_logging_log_group" "function_log_group" {
  compartment_id = local.compartment_ocid
  display_name   = local.function_app_log_group_name
  description    = "Log group for logging function logs"
  freeform_tags  = local.freeform_tags
}

#Resource for creating a log  and associate it with the function
resource "oci_logging_log" "function_execution_log" {
  depends_on   = [oci_functions_function.logging_function]
  display_name = local.function_app_log_name
  log_group_id = oci_logging_log_group.function_log_group.id
  log_type     = "SERVICE"
  configuration {
    source {
      category    = "invoke"
      resource    = oci_functions_application.logging_function_app.id
      service     = "functions"
      source_type = "OCISERVICE"
    }
    compartment_id = local.compartment_ocid
  }
  is_enabled    = true
  freeform_tags = local.freeform_tags
}

# Resource for the function
resource "oci_functions_function" "logging_function" {
  depends_on = [oci_functions_application.logging_function_app]

  application_id     = oci_functions_application.logging_function_app.id
  display_name       = local.function_name
  memory_in_mbs      = local.memory_in_mbs
  timeout_in_seconds = local.time_out_in_seconds

  defined_tags  = {}
  freeform_tags = local.freeform_tags
  image         = local.image_url
}

# Service Connector Hub - Routes logs from multiple log groups to New Relic function
resource "oci_sch_service_connector" "nr_logging_service_connector" {
  for_each = local.connectors_map

  compartment_id = local.compartment_ocid
  display_name   = each.value.display_name
  description    = "Connectors to send logs data to Newrelic"
  freeform_tags  = local.freeform_tags

  source {
    kind = "logging"
    dynamic "log_sources" {
      for_each = each.value.log_sources
      content {
        compartment_id = log_sources.value.compartment_id
        log_group_id   = log_sources.value.log_group_id
      }
    }
  }

  target {
    kind              = "functions"
    batch_size_in_kbs = local.batch_size_in_kbs
    batch_time_in_sec = local.batch_time_in_sec
    compartment_id    = local.compartment_ocid
    function_id       = oci_functions_function.logging_function.id
  }

  depends_on = [oci_functions_function.logging_function]
}


module "vcn" {
  source                   = "oracle-terraform-modules/vcn/oci"
  version                  = "3.6.0"
  count                    = var.create_vcn ? 1 : 0
  compartment_id           = local.compartment_ocid
  defined_tags             = {}
  freeform_tags            = local.freeform_tags
  vcn_cidrs                = ["10.0.0.0/16"]
  vcn_dns_label            = "nrlogging"
  vcn_name                 = local.vcn_name
  lockdown_default_seclist = false
  subnets = {
    private = {
      cidr_block = "10.0.0.0/16"
      type       = "private"
      name       = local.subnet
    }
  }
  create_nat_gateway            = true
  nat_gateway_display_name      = local.nat_gateway
  create_service_gateway        = true
  service_gateway_display_name  = local.service_gateway
  create_internet_gateway       = true                   # Enable creation of Internet Gateway
  internet_gateway_display_name = local.internet_gateway # Name the Internet Gateway
}

data "oci_core_route_tables" "default_vcn_route_table" {
  depends_on     = [module.vcn] # Ensure VCN is created before attempting to find its route tables
  count          = var.create_vcn ? 1 : 0
  compartment_id = local.compartment_ocid
  vcn_id         = module.vcn[0].vcn_id

  filter {
    name   = "display_name"
    values = ["Default Route Table for ${local.vcn_name}"]
    regex  = false
  }
}

# Resource to manage the VCN's default route table and add your rule.
resource "oci_core_default_route_table" "default_internet_route" {
  manage_default_resource_id = data.oci_core_route_tables.default_vcn_route_table[0].route_tables[0].id
  depends_on = [
    module.vcn,
    data.oci_core_route_tables.default_vcn_route_table
  ]
  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = module.vcn[0].internet_gateway_id # Reference the internet gateway created by the module
    description       = "Route to Internet Gateway for New Relic logging"
  }

}

output "vcn_network_details" {
  depends_on  = [module.vcn]
  description = "Output of the created network infra"
  value = var.create_vcn && length(module.vcn) > 0 ? {
    vcn_id             = module.vcn[0].vcn_id
    nat_gateway_id     = module.vcn[0].nat_gateway_id
    nat_route_id       = module.vcn[0].nat_route_id
    service_gateway_id = module.vcn[0].service_gateway_id
    sgw_route_id       = module.vcn[0].sgw_route_id
    subnet_id          = module.vcn[0].subnet_id[local.subnet]
    } : {
    vcn_id             = ""
    nat_gateway_id     = ""
    nat_route_id       = ""
    service_gateway_id = ""
    sgw_route_id       = ""
    subnet_id          = var.function_subnet_id
  }
}

output "stack_id" {
  value = data.oci_resourcemanager_stacks.current_stack.stacks[0].id
}

# Resource to update the New Relic stackId in NRDB
resource "null_resource" "newrelic_link_account" {
  depends_on = [oci_functions_function.logging_function, oci_sch_service_connector.nr_logging_service_connector]
  provisioner "local-exec" {
    command = <<EOT
      # Main execution for cloudLinkAccount
      response=$(curl --silent --request POST \
        --url "${local.newrelic_graphql_endpoint}" \
        --header "API-Key: ${local.user_api_key}" \
        --header "Content-Type: application/json" \
        --header "User-Agent: insomnia/11.1.0" \
        --data '${jsonencode({
    query = local.updateLinkAccount_graphql_query
})}')

      # Log the full response for debugging
      echo "Full Response: $response"

      # Combine errors
      errors="$root_errors"$'\n'"$account_errors"

      # Check if errors exist
      if [ -n "$errors" ] && [ "$errors" != $'\n' ]; then
        echo "Operation failed with the following errors:" >&2
        echo "$errors" | while IFS= read -r error; do
          echo "- $error" >&2
        done
        exit 1
      fi

    EOT
}
}