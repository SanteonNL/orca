Frontend
########

Code: https://github.com/SanteonNL/orca/tree/main/frontend

Configuration
*************

.. list-table:: Frontend Config (exposed to the Web)
    :header-rows: 1

    * - Environment Variable
      - Default
      - Description
    * - ``NEXT_PUBLIC_BASE_PATH``
      - 
      - Sets the Next.js `basePath <https://nextjs.org/docs/app/api-reference/next-config-js/basePath>`_

.. list-table:: Frontend Config (private)
    :header-rows: 1

    * - Environment Variable
      - Default
      - Description
    * - ``NODE_ENV``
      -
      - This is also used to switch CPS/EHR FHIR endpoints
    * - ``NEXT_TELEMETRY_DISABLED``
      -
      - Allows to opt-out from Next.js `Telemetry <https://nextjs.org/telemetry>`_ collection
    * - ``TERMINOLOGY_SERVER_BASE_URL``
      -
      - National terminology server FHIR URL (For the Dutch terminology server, see: https://nictiz.nl/publicaties/nationale-terminologie-server-handleiding-voor-nieuwe-gebruikers/)
    * - ``TERMINOLOGY_SERVER_USERNAME``
      -
      - National terminology server username
    * - ``TERMINOLOGY_SERVER_PASSWORD``
      -
      - National terminology server password

.. list-table:: Hardcoded/Indirect Config
  :header-rows: 1

  * - Environment Variable
    - Default
    - Description
  * -
    - ``https://terminologieserver.nl/auth/realms/nictiz/.well-known/openid-configuration``
    - National terminology server OpenID configuration endpoint
  * - 
    - ``cli_client``
    - National terminology server OpenID Client ID
  * -
    - ``http://localhost:9090/fhir``
    - EHR FHIR URL, if ``NODE_ENV !== "production"``
  * -
    - ``${window.location.origin}/orca/contrib/ehr/fhir``
    - EHR FHIR URL, if ``NODE_ENV === "production"``
  * -
    - ``http://localhost:7090/fhir``
    - CPS FHIR URL, if ``NODE_ENV !== "production"``
  * -
    - ``${window.location.origin}/orca/contrib/cps/fhir``
    - CPS FHIR URL, if ``NODE_ENV === "production"``
