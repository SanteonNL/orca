.. _getting-started_overview:

Overview
########

Roles
*****

ORCA provides a reference implementation for the two roles defined by the `Shared Care Planning <https://santeonnl.github.io/shared-care-planning/>`_ (SCP) specification:

- `Care Plan Contributor <https://santeonnl.github.io/shared-care-planning/overview.html#care-plan-contributor>`_ (CPC) role allows SCP participants to access EHRs with authorization based on Care Plan membership
- `Care Plan Service <https://santeonnl.github.io/shared-care-planning/overview.html#care-plan-service>`_ (CPS) role manages Care Plans

The following depicts an OrORCAca setup providing both the CPC role and the optional CPS role. *(Nuts, is not a SCP role, but a trust layer used for asserting identity of SCP participants.)*

.. image:: ../../_static/images/Shared\ Care\ Planning\ Network-CPC+CPS.drawio.svg
    :alt: Typical Shared Care Planning Network

Only a single CPS is required to manage the Care Plans, so most participants in Shared Care Planning only need to implement the CPC role


Typical Shared Care Planning Network
************************************

In a typical SCP network, one party takes the responsibility for running a CPS role, this party is likely to participate in SCP and would probably implement both the CPS and CPC roles. Other parties only need to implement the CPC role.

A typical SCP network would consist of various hospitals, general practitioners, medical service centers, and home care organizations, all joining in Shared Care Planning by implementing the CPC role for their Electronic Health Records (EHR) systems.

.. image:: ../../_static/images/Shared\ Care\ Planning\ Network-Network.drawio.svg
    :alt: Typical Shared Care Planning Network

Components
**********

Each ORCA deployment contains at least the following two components:

- **Orchestrator:** provides the reference implementations of the CPC and CPS roles
- **Frontend:** provides a frontend for ORCA, allows filling of Questionnaires
- **Nuts:** provides a distributed trust network, which is used in Shared Care Planning to obtain authentication tokens for authentication to other Shared Care Planning participants (see `Nuts <https://nuts.nl/>`_)
- **Nuts Admin:** provides an admin interface for Nuts

For testing and demo purposes, also the following components are provided:

- **Hospital Simulator:** simulates an Hospital's Electronic Health Records system that wants to participate in Shared Care Planning
- **Viewer Simulator:** simulates an Medical Serivce Center's Electronic Health Records system that wants to participate in Shared Care Planning
