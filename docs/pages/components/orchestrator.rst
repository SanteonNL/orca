.. _components-orchestrator:

Orchestrator
############

Configuration
*************

.. list-table:: Orchestrator Config
    :header-rows: 1

    * - Environment Variable
      - Default
      - Description
    * - ``ORCA_NUTS_API_URL``
      -
      - private API for this organizations' Nuts node
    * - ``ORCA_NUTS_PUBLIC_URL``
      -
      - public API for this organizations' Nuts node
    * - ``ORCA_NUTS_SUBJECT``
      -
      - Own Subject  **FIXME: Better Description**
    * - ``ORCA_PUBLIC_ADDRESS``
      - ``:8080``
      - Address to listen on
    * - ``ORCA_PUBLIC_URL``
      - ``/``
      - base URL of the service, set it in case the service is behind a reverse proxy that maps it to a different URL than root (``/``)
    * - ``ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL``
      -
      - base URL of the CarePlanService at which the CarePlanContributor creates/reads CarePlans
    * - ``ORCA_CAREPLANCONTRIBUTOR_FRONTEND_URL``
      -
      - base URL of the Orca Frontend
    * - ``ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_CLIENTID``
      -
      - 
    * - ``ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_CLIENTSECRET``
      -
      - 
    * - ``ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_REDIRECTURI``
      -
      - 
    * - ``ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_SCOPE``
      -
      - 
    * - ``ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_DEMO_ENABLED``
      -
      - 
    * - ``ORCA_CAREPLANCONTRIBUTOR_FHIR_URL``
      -
      - base URL of the (EHR) FHIR server
    * - ``ORCA_CAREPLANCONTRIBUTOR_FHIR_AUTH_TYPE``
      -
      -  Authentication to use. Supported options: ``azure-managedidentity``. Leave empty for no authentication
    * - ``ORCA_CAREPLANCONTRIBUTOR_FHIR_AUTH_SCOPES``
      -
      -
    * - ``ORCA_CAREPLANCONTRIBUTOR_ENABLED``
      - ``true``
      - enable the CPC role
    * - ``ORCA_CAREPLANSERVICE_FHIR_URL``
      -
      - base URL of the (Care Plan Service) FHIR server
    * - ``ORCA_CAREPLANSERVICE_FHIR_AUTH_TYPE``
      -
      -  Authentication to use. Supported options: ``azure-managedidentity``. Leave empty for no authentication
    * - ``ORCA_CAREPLANSERVICE_FHIR_AUTH_SCOPES``
      -
      -
    * - ``ORCA_CAREPLANSERVICE_ENABLED``
      - ``true``
      - enable the CPC role
    * - ``AZURE_CLIENT_ID``
      -
      - For ``azure-managedidentity`` specify the UserAssignedManagedIdentity
      

Endpoints
*********

.. list-table:: Orca Endpoints
    :header-rows: 1

    * - Method
      - Path
      - Description
    * - ``GET``
      - ``/health``
      - Health Endpoint

.. list-table:: Care Plan Service role Endpoints
    :header-rows: 1

    * - Method
      - Path
      - Description
    * - ``POST``
      - ``/cps/Task``
      - Create new Task
    * - ``PUT``
      - ``/cps/Task/{id}``
      - Update Task
    * - ``POST``
      - ``/cps/CarePlan``
      - Create new CarePlan
    * - ``*``
      - ``/*``
      - Proxy to CPS FHIR store

.. list-table:: Care Plan Contributor role Endpoints
    :header-rows: 1

    * - Method
      - Path
      - Description
    * - ``POST``
      - ``/contrib/fhir/notify``
      - Handles incoming FHIR notifications
    * - ``GET``
      - ``/contrib/fhir/*`` (future ``/cpc/*``)
      - Proxies request to ????? EHR after authorization based on the CarePlan
    * - ``GET``
      - ``/contrib/context``
      - Returns 
    * - ``GET``
      - ``/contrib/patient``
      - 
    * - ``GET``
      - ``/contrib/practitioner``
      - 
    * - ``GET``
      - ``/contrib/serviceRequest``
      - 
    * - ``GET``
      - ``/contrib/ehr/fhir/*``
      - 
    * - ``GET``
      - ``/contrib/cps/fhir/*``
      - 
    * - ``GET``
      - ``/contrib/``
      - Redirect to Frontend
    * - ``GET``
      - ``/smart-app-launch``
      - Handle smart app launch (SMART on FHIR)
    * - ``GET``
      - ``/smart-app-launch/redirect``
      - Handle smart app launch redirect (SMART on FHIR)
    * - ``*``
      - ``/demo-app-launch``
      - Handle smart app launch redirect (SMART on FHIR)