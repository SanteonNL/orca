.. _deployment-development:

Development
###########

For development, you can run everything in Docker on your local machine.
For everything to have public endpoints secured with TLS, Azure Dev Tunnels are created.


Prerequisites
*************

- `Docker <https://www.docker.com/products/docker-desktop/>`_
- `Azure Dev Tunnels CLI <https://learn.microsoft.com/en-us/azure/developer/dev-tunnels/get-started?tabs=macos>`_

Login to Azure with the Dev Tunnels CLI:

.. code-block:: bash

    $ devtunnel user login


Start Development Environment
*****************************

.. code-block:: bash

    $ cd deployments/dev/
    $ ./start.sh
