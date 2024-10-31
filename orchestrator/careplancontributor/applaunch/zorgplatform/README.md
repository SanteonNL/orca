# Zorgplatform SSO Launch
Launch implementation according to the Zorgplatform/Chipsoft SSO specs

### Access via local deployment
1. `az login`
2. configure the kv values, e.g:
   ```json 
   {
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_ENABLED": "true",
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_SIGN_ISS": "<iss>", //The hl7 oid of the care organization configured in Zorgplatform
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_SIGN_AUD": "<aud>", //The STS URL
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_DECRYPT_ISS": "<iss>", //The STS URL
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_DECRYPT_AUD": "<aud>", //The service URL configured in Zorgplatform
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_BASEURL": "<url>", //https://zorgplatform.online OR https://acceptatie.zorgplatform.online
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_STSURL": "<url>", //https://zorgplatform.online/sts OR https://acceptatie.zorgplatform.online/sts
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_APIURL": "<url>", //https://api.zorgplatform.online OR https://api.acceptatie.zorgplatform.online
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_URL": "<url>", //The URL of the Azure KeyVault to use
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_CREDENTIALTYPE": "<type>", //The Azure credential type, "default", "cli" or "managed_identity"
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_DECRYPTCERTNAME": "<certname>", //Name of the KV decrypt certificate (used to decrypt assertions that are received from Zorgplatform)
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_SIGNCERTNAME": "<certname>", //Name of the KV signing certificate (used to sign assertions that wil be sent to Zorgplatform)
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_CLIENTCERTNAME": "<certname>", //Name of the KV client certificate (used to set up mTLS with Zorgplatform)
   }
   ```