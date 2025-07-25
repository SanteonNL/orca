# Orca
ORCA (**O**pen source **R**eference-implementation shared **CA**replanning) implements the [Shared Care Planning specification](https://santeonnl.github.io/shared-care-planning/).

## Features
ORCA implements the following features:

- Initiating a FHIR workflow (Task placer) and handling incoming workflow Tasks (Task filler), with/from other care organizations through Shared Care Planning.
- User interface for care professionals to fill in questionnaires required for a FHIR workflow.
- Proxy for the care organization's EHR to access the Care Plan Service's FHIR API, that handles authentication.
- Proxy for the care organization's EHR to access the other Shared Care Planning participants' FHIR API, that handles localization, authentication and data aggregation for participants that use the ChipSoft Zorgplatform FHIR API.

The following features are planned:
- Lightweight decision engine for accepting FHIR workflow Tasks (Task filler).
- Proxy for the care organization's EHR to access the other Shared Care Planning participants' FHIR API, that handles localization, authentication and data aggregation for participants that use the FHIR API of Azure Health Data Services. Please contribute to the Orca project if you want to prioritize the inclusion of a particular FHIR API.

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
    * **Orchestrator** fills in 2 roles:
      1. *Care Plan Contributor*: provides the EHR with access to remote *Care Plan Service* and *Care Plan Contributor* FHIR APIs.
         Provides external Care Plan Contributors access to the local EHR's FHIR API. Negotiates new Tasks using Questionnaires.
         It provides the following APIs:
         - Internal-facing FHIR API for the local EHR to access the Care Plan Service and other organization's FHIR APIs.
           This FHIR API notifies the local EHR of updates using FHIR subscription (DECIDE ON THIS).
         - Internal-facing API for the local EHR to launch the *Frontend* web application.
         - External-facing FHIR API for other SCP nodes to access the local EHR's data.
      2. *Care Plan Service* (optional): manages CarePlans and CareTeams according to Shared Care Planning.
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

#### Workflow initiation/acceptance
When a care professional wants to initiate a FHIR Task for another care organization, they start a new FHIR workflow [according to Shared Care Planning](https://santeonnl.github.io/shared-care-planning/overview.html#creating-and-responding-to-a-task).

The process is as follows:
1. Placer: the care professional, using the *Frontend*, chooses to create a new Task for a specified Condition.
2. Placer: *Frontend* creates a new FHIR Task at the Care Plan Service through ORCA's *Orchestrator*.
3. At this point, the Care Plan Service notifies the Task filler that a new Task is available.
4. The Task filler (using ORCA's *Task Engine*) and placer (using ORCA's *Frontend*) now negotiate the Task details:
   1. Filler: if the filler needs more information, it adds a sub-Task containing a Questionnaire to the `Task.input` (refer to the SCP specification for more details).
   2. Placer: responds by filling the Questionnare, adding a QuestionnaireResponse to the sub-Task's output (refer to the SCP specification for more details).
   3. Filler: verifies the QuestionnaireResponse either:
      - Accept the Task if it is able and willing to perform it.
      - Add another Questionnaire to the `Task.input` if it needs more information.
      - Reject the Task if it cannot/won't perform it.
5. Filler: ORCA's *Orchestrator* Task Engine notifies the filler's EHR about the accepted Task.
6. Placer: *Frontend* informs the care professional that the filler has accepted the Task. 

Whenever a Task is created/updated at the Care Plan Service, it notifies the other Task participant.
The notification is then handled to perform the Task negotiation:
- Filler: notification is received by ORCA's *Orchestrator* and handled by the task engine.
  The task engine then decides whether to accept the Task, and if not, what additional information is required.
- Placer: notification is received by ORCA's *Orchestrator* and handled by the task engine.

### FHIR patient data access
Participants of a CarePlan can query each other's FHIR APIs to access the patient's data related to that CarePlan, [according to Shared Care Planning](https://santeonnl.github.io/shared-care-planning/overview.html#getting-data-from-careteam-members).

The party querying the data is called the *Requester*, and the party providing the data is called the *Holder*.

The process is as follows:
1. Requester: the care professional using their EHR chooses a patient of which they want to access the data.
2. Requester: the EHR queries the list of CarePlans available at the Care Plan Service for the patient through ORCA's *Orchestrator*.
3. Requester: the care professional chooses a CarePlan and uses the EHR to query FHIR resources through ORCA's *Orchestrator*.
4. Requester: ORCA's *Orchestrator* performs the query at each of the CarePlan's CareTeam participants:
   1. Requester: *Orchestrator* looks up the DID of the CareTeam participant giving its URA, then resolves the endpoints through its DID document:
      1. Use local Nuts node to lookup Holder's DID in the SCP Discovery Service using the Holder's URA number.
      2. Use local Nuts node to resolve the service endpoints from Holder's DID document.
   2. Requester: *Orchestrator* uses the local Nuts node to request an access token from the remote SCP node.
      1. Holder: validates the authorization request and returns an access token.
   3. Requester: *Orchestrator* uses the access token to query the remote SCP node's FHIR API.
      1. Requester: *Orchestrator* forwards the FHIR query to the remote SCP node's FHIR API.
      2. Holder: authenticates and authorizes the request, and returns the FHIR resources.
5. Requester: *Orchestrator* collects the resulting FHIR resources into a Bundle and returns them to the EHR.

## Integration
When integrating with the ORCA system, the EHR (and its FHIR API) needs to support the following integrations:

- Allow *Orchestrator* access to the FHIR API
  - Supported means of authentication: Azure Managed Identity, no authentication
- To receive new/updated Task notifications: 
  - Provide message broker queue at which *Orchestrator* can notify the EHR of accepted/updated Tasks.
- Invoking *Orchestrator*'s internal-facing FHIR API (to list Shared CarePlans and query remote FHIR resources)
- *OPTIONAL*: If using the *Frontend*, invoking *Orchestrator*'s app launch
  - Supported: ChipSoft HIX, SMART on FHIR (TODO)

## Deployment
Note: this section needs to be expanded.

When deploying ORCA, you need to supply the following components yourself:

- An HL7 FHIR R4 API that allows *Orchestrator* to access the local EHR's data for answering queries from other CareTeam participants.
  - Supported: Microsoft Azure Health FHIR API, HAPI FHIR.
- Key storage for the Orchestrator and Nuts node
  - Supported: Azure Key Vault, on-disk (not recommended).
- SQL database for the Nuts node to store its data (refer to Nuts node documentation for details).
