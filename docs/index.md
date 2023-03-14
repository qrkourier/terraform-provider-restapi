---
page_title: "OpenZiti restapi Provider"
subcategory: "Utility"
description: |-
    Ultra-thin REST wrapper for the OpenZiti Management API forked from Mastercard/restapi

---

# OpenZiti Managment restapi Provider

## A Note about Terraform Provider Configuration

You'll probably need to get the provider configuration from remote state provided by another plan. This is because provider configuration occurs in an early phase of plan and apply, and so it's not possible to reliably obtain the configuration values from the same plan.

## Example Usage

```go
terraform {
    cloud {}
    required_providers {
        restapi = {
            source = "qrkourier/restapi"
            version = "~> 1.23.0"
        }
    }
}

data "terraform_remote_state" "controller_state" {
    backend = "remote"
    config = {
        organization = "acmeorg"
        workspaces = {
            name = "acmespace"
        }
    }
}

provider restapi {
    uri                   = "https://${data.terraform_remote_state.controller_state.outputs.ziti_controller_mgmt_external_host}:443/edge/management/v1"
    cacerts_string        = (data.terraform_remote_state.controller_state.outputs.ctrl_plane_cas).data["ctrl-plane-cas.crt"]
    ziti_username         = (data.terraform_remote_state.controller_state.outputs.ziti_admin_password).data["admin-user"]
    ziti_password         = (data.terraform_remote_state.controller_state.outputs.ziti_admin_password).data["admin-password"]
}
```

## OpenZiti Authentication

You must provide at least one of `cacerts_file` or `cacerts_string` with the OpenZiti Controller's CA bundle as PEM
You must provide at least one of (`ziti_username` and `ziti_password`) or ((`cert_file` or `cert_string`) and (`key_file` or `key_string`)).

You must have an `updb` Authenticator to use password auth.
You must have a `cert` Authenticator and compatible Authentication Policy to use cert auth.

## Schema

### Required

- **uri** (String, Required) URI of the REST API endpoint. This serves as the base of all requests.

### Optional

- **cacerts_string** (String, Optional) OpenZiti Controller's CA bundle as PEM
- **cacerts_file** (String, Optional) file path to OpenZiti Controller's CA bundle as PEM
- **ziti_username** (String, Optional) When set, will use this username for OpenZiti password auth to the API.
- **ziti_password** (String, Optional) When set, will use this password for OpenZiti password auth to the API.
- **cert_file** (String, Optional) When set with the key_file parameter, the provider will load a client certificate as a file for mTLS authentication.
- **cert_string** (String, Optional) When set with the key_string parameter, the provider will load a client certificate as a string for mTLS authentication.
- **key_file** (String, Optional) When set with the cert_file parameter, the provider will load a client certificate as a file for mTLS authentication. Note that this mechanism simply delegates to golang's tls.LoadX509KeyPair which does not support passphrase protected private keys. The most robust security protections available to the key_file are simple file system permissions.
- **key_string** (String, Optional) When set with the cert_string parameter, the provider will load a client certificate as a string for mTLS authentication. Note that this mechanism simply delegates to golang's tls.LoadX509KeyPair which does not support passphrase protected private keys. The most robust security protections available to the key_file are simple file system permissions.
- **copy_keys** (List of String, Optional) When set, any PUT to the API for an object will copy these keys from the data the provider has gathered about the object. This is useful if internal API information must also be provided with updates, such as the revision of the object.
- **create_method** (String, Optional) Defaults to `POST`. The HTTP method used to CREATE objects of this type on the API server.
- **create_returns_object** (Boolean, Optional) Set this when the API returns the object created only on creation operations (POST). This is used by the provider to refresh internal data structures.
- **debug** (Boolean, Optional) Enabling this will cause lots of debug information to be printed to STDOUT by the API client.
- **destroy_method** (String, Optional) Defaults to `DELETE`. The HTTP method used to DELETE objects of this type on the API server.
- **headers** (Map of String, Optional) A map of header names and values to set on all outbound requests. This is useful if you want to use a script via the 'external' provider or provide a pre-approved token or change Content-Type from `application/json`. If `username` and `password` are set and Authorization is one of the headers defined here, the BASIC auth credentials take precedence.
- **id_attribute** (String, Optional) When set, this key will be used to operate on REST objects. For example, if the ID is set to 'name', changes to the API object will be to http://foo.com/bar/VALUE_OF_NAME. This value may also be a '/'-delimited path to the id attribute if it is multiple levels deep in the data (such as `attributes/id` in the case of an object `{ "attributes": { "id": 1234 }, "config": { "name": "foo", "something": "bar"}}`
- **insecure** (Boolean, Optional) When using https, this disables TLS verification of the host.
- **oauth_client_credentials** (Block List, Max: 1) Configuration for oauth client credential flow (see [below for nested schema](#nestedblock--oauth_client_credentials))
- **rate_limit** (Number, Optional) Set this to limit the number of requests per second made to the API.
- **read_method** (String, Optional) Defaults to `GET`. The HTTP method used to READ objects of this type on the API server.
- **test_path** (String, Optional) If set, the provider will issue a read_method request to this path after instantiation requiring a 200 OK response before proceeding. This is useful if your API provides a no-op endpoint that can signal if this provider is configured correctly. Response data will be ignored.
- **timeout** (Number, Optional) When set, will cause requests taking longer than this time (in seconds) to be aborted.
- **update_method** (String, Optional) Defaults to `PUT`. The HTTP method used to UPDATE objects of this type on the API server.
- **use_cookies** (Boolean, Optional) Enable cookie jar to persist session.
- **username** (String, Optional) When set, will use this username for BASIC auth to the API.
- **password** (String, Optional) When set, will use this password for BASIC auth to the API.
- **write_returns_object** (Boolean, Optional) Set this when the API returns the object created on all write operations (POST, PUT). This is used by the provider to refresh internal data structures.
- **xssi_prefix** (String, Optional) Trim the xssi prefix from response string, if present, before parsing.

<a id="nestedblock--oauth_client_credentials"></a>
### Nested Schema for `oauth_client_credentials`

Required:

- **oauth_client_id** (String, Required) client id
- **oauth_client_secret** (String, Required) client secret
- **oauth_token_endpoint** (String, Required) oauth token endpoint

Optional:

- **endpoint_params** (Map of List of String, Optional) Additional key/values to pass to the underlying Oauth client library (as EndpointParams)
- **oauth_scopes** (List of String, Optional) scopes
