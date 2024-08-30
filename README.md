# Orca
Open source Reference-implementation CAreplanning.

## Features
ORCA implements the following features:

- Initiating a FHIR workflow (Task placer) and handling incoming workflow Tasks (Task filler), with/from other care organizations through Shared Care Planning.
- User interface for care professionals to fill in questionnaires required for a FHIR workflow.
- Proxy for the care organization's EHR to access the Care Plan Service's FHIR API, that handles authentication. 

The following features are planned:
- Lightweight decision engine for accepting FHIR workflow Tasks (Task filler).
- Proxy for the care organization's EHR to access the other Shared Care Planning participants' FHIR API, that handles localization, authentication and data aggregation.

## Architecture
Systems that implement Shared Care Planning are known as SCP nodes. ORCA is such an implementation.
Shared Care Planning (SCP) participant can deploy ORCA invoke FHIR workflow Tasks to other SCP participants,
use it to access external FHIR APIs secured with SCP and handle incoming FHIR requests from other SCP participants.
It can also act as Care Plan Service, to manage CarePlans according to Shared Care Planning.

An ORCA instance communicates to other SCP nodes (Care Plan Contributors and/or Services) according to Shared Care Planning.
Other participants might or might not use ORCA as SCP node, but they must implement the Shared Care Planning specification.

### Overview
This high-level diagram describes components in ORCA and their role.

![system-diagram.svg](docs/images/system-diagram.svg)

* **Local Care Provider**: a care organization using ORCA to participate in Shared Care Planning.
  * **XIS** 
    * **EHR** is the care organization's EHR system, which care professionals use in their day-to-day work. It is used to initiate FHIR workflows by launching the *ORCA Frontend*. 
    * **FHIR API** is the FHIR API of the care organization's EHR system, used by ORCA to access the care organization's data.
  * **ORCA**
    * **Frontend** is the user interface for care professionals to fill in questionnaires required for a FHIR workflow, and to view shared CarePlans and patient data from remote care organizations.
    * **Orchestrator** can fill in 2 roles: 1) CarePlanContributor that proxies FHIR calls from the Frontend and local EHR to the remote CarePlanServer, and 2) CarePlanService that handles manages CarePlans according to Shared Care Planning.
    * **Authorization Server** TODO
* **External Care Provider**: another care organization participating in Shared Care Planning.
  * **Care Plan Service** TODO
  * **Care Plan Contributor** TODO
  * **Authorization Server** TODO

## Integration

