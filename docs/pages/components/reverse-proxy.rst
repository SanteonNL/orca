.. _components-reverse-proxy:

Reverse Proxy
#############

An (API) Gateway or other kind of reverse-proxy should be used in front of the ORCA modules.
The following mapping is suggested.

.. list-table:: Reverse Proxy Mapping
    :header-rows: 1

    * - Path
      - Target Component
      - Healthcheck
      - Authentication
    * - ``/orca/``
      - :doc:`/pages/components/orchestrator`
      - ``/health``
      - mTLS
    * - ``/frontend/``
      - :doc:`/pages/components/frontend`
      - ``/frontend/api/health``
      - FIXME: mTLS?
    * - ``/nuts/``
      - :doc:`/pages/components/nuts` (Public API)
      - ``/health`` (Internal API!)
      - public
    * - ``/.well-known/oauth-authorization-server/nuts.*``
      - :doc:`/pages/components/nuts` (Public API)
      - ``/health`` (Internal API!)
      - public
    * - ``/admin/``
      - :doc:`/pages/components/nuts-admin`
      - ``/``
      - FIXME: public?

In test environments you might also want to expose the simulators, in that case we suggest the following mapping.

.. list-table:: Simulator Proxy Mapping
    :header-rows: 1

    * - Path
      - Target Component
      - Healthcheck
      - Authentication
    * - ``/ehr/``
      - :doc:`/pages/components/hospital-simulator`
      - ``/ehr/api/health``
      - FIXME: public?
    * - ``/viewer/``
      - :doc:`/pages/components/viewer-simulator`
      - ``/viewer/api/health``
      - FIXME: public?
