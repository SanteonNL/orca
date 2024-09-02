# Orca
ORCA (**O**pen source **R**eference-implementation shared **CA**replanning) implements the [Shared Care Planning specification](https://santeonnl.github.io/shared-care-planning/).

## Features
ORCA implements the following features:

- Initiating a FHIR workflow (Task placer) and handling incoming workflow Tasks (Task filler), with/from other care organizations through Shared Care Planning.
- User interface for care professionals to fill in questionnaires required for a FHIR workflow.
- Proxy for the care organization's EHR to access the Care Plan Service's FHIR API, that handles authentication. 

The following features are planned:
- Lightweight decision engine for accepting FHIR workflow Tasks (Task filler).
- Proxy for the care organization's EHR to access the other Shared Care Planning participants' FHIR API, that handles localization, authentication and data aggregation.

## Architecture
Systems that implement Shared Care Planning (SCP) are known as SCP nodes. ORCA is such an implementation.
SCP participants can deploy ORCA to enable SCP features:

- invoke FHIR workflow Tasks to other participants (Care Plan Contributors, CPC),
- use it to access FHIR APIs of other SCP nodes secured,
- handle incoming FHIR requests from other SCP participants,
- host a Care Plan Service (CPS), at which other CPCs can create CarePlans.

An ORCA instance communicates to other SCP nodes (Care Plan Contributors and/or Care Plan Services) according to Shared Care Planning to enable these features.
Other participants might or might not use ORCA as SCP node, but they must implement the Shared Care Planning specification.

### Overview
This high-level diagram describes components in ORCA and their role.

![system-diagram.svg](docs/images/system-diagram.svg)

* **Local Care Provider**: a care organization using ORCA to participate in Shared Care Planning.
  * **XIS** 
    * **EHR** is the care organization's EHR system, which care professionals use in their day-to-day work. It is used to initiate FHIR workflows by launching the *Frontend*. 
    * **FHIR API** is the FHIR API of the care organization's EHR system, used by ORCA to access the care organization's data.
  * **ORCA**
    * **Frontend** is a web application for care professionals to fill in questionnaires required for a FHIR workflow, and to view shared CarePlans and patient data from remote care organizations.
      It uses the FHIR APIs proxies by the *Orchestrator*.
    * **Orchestrator** can fill in 2 roles: 1) *Care Plan Contributor* that proxies FHIR calls from the *Frontend* and local EHR to the remote *Care Plan Service*, and authorizes incoming FHIR requests from other CPCs,
      and 2) Care Plan Service that handles manages CarePlans according to Shared Care Planning.
    * **Authorization Server** is the OAuth2 server that other SCP nodes use to authenticate to the local ORCA instance, and that ORCA uses to authenticate to other SCP nodes.
* **External Care Provider**: another care organization participating in Shared Care Planning.
  * **Care Plan Service** is used by *Care Plan Contributor*s to create CarePlans, and manages CareTeams according to those CarePlans. 
  * **Care Plan Contributor** provides access to its FHIR API to the local ORCA instance, and access the local FHIR API through ORCA.
  * **Authorization Server** (see above)

### Transactions
This section describes how the Shared Care Planning transactions are implemented in ORCA.

#### Creating a new CarePlan
1. The care professional using the organization's EHR, selects a patient and launches the ORCA *Frontend* web application through *Orchestrator*.
2. *Frontend* retrieves and shows the following data through ORCA's *Orchestrator*:
   - the launch context: the FHIR Patient, ServiceRequest, and PractitionerRole resources.
     This is retrieved through *Orchestrator* from the local EHR's FHIR API.
   - the existing CarePlans for the patient.
     This is retrieved through *Orchestrator* from the *Care Plan Service*, which is either local or remote.
3. The care professional inputs the CarePlan details (name, condition) and submits.
4. The *Frontend* creates the new CarePlan resource at the *Care Plan Service* through ORCA *Orchestrator*. 

#### Initiating a workflow
When a care professional wants to initiate a FHIR Task for another care organization, they start a new FHIR workflow.
As the remote care organization, the Task filler, might 

#### Accepting a workflow Task

## Integration


