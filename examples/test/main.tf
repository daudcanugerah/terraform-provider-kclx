terraform {
  required_providers {
    kcl = {
      source = "daudcanugerah/kcl"
    }
  }
}

provider "kcl" {
  kcl_path = "/usr/local/bin/kcl"
}

resource "kcl_exec" "config" {
  source_dir = "./kcl-configs"
  
  args = [
    "-o", "output.json",
    "-d",  # Debug mode
  ]
  
  triggers = {
    config_hash = filemd5("${path.module}/kcl-configs/main.k")
    timestamp   = timestamp()
  }
  
  timeout = 600  # 10 minutes
  
  environment = {
    KCL_ENV      = "production"
    CUSTOM_VALUE = "12345"
  }
}

output "kcl_output" {
  value = kcl_exec.config.output
}
