# Zorgplatform SSO Launch
Launch implementation according to the Zorgplatform/Chipsoft SSO specs

# TODO
Make the README clear, currently mostly contains steps I ran into during development

## Requirements
1. Azure KV certs need to be imported with the [decrypt operation enabled](https://stackoverflow.com/a/55719562)

### Access to kv in Azure
1. activate access in Azure PIM (Privileged Identity Management) for the `ProductionAdministrators` Group
2. Requires MFA step-up
3. Go to the keyvault you wish to use 
4. Go to Settings > Networking
5. Whitelist your IPv4 IP address
6. After this you should be able to see the certificates

### Access via local deployment
1. `az login`
2. configure the kv values, e.g:
   ```json 
   {
    "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_ENABLED": "true",
    "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_ISS": "<iss>", //The hl7 oid of the care organization configured in Zorgplatform
    "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AUD": "<aud>", //The service URL configured in Zorgplatform
    "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_URL": "<kv_url>", //The URL of the Azure KeyVault to use
    "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_DECRYPTCERTNAME": "<certname>", //The name of the cert inside the configured Azure KeyVault
   }
   ```