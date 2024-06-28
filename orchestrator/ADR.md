# Architectural Decision Records
Here, Architectural Decisions are documented. 

## Care Plan Service

### Notification Delivery

The Care Plan Service needs to deliver notifications to Care Plan Contributors:
- When a new Task is added to a CarePlan, the filler must be notified. Otherwise, the filler does not know there's a Task to fill (practically loss of data).
- When a Task is updated, the placer and owner should be notified.

These 2 notifications are subject to the following requirements:
- The first notification is essential (otherwise the filler might never know about a Task to be accepted), so requires guaranteed delivery for the transaction to succeed.
  This is especially important, since at the placer's side there's a person (doctor/nurse) waiting for the filler to accept the Task.
- The second notification (on update) could be delivered best-effort, as Care Plan Contributors already have a reference to the Task/CarePlan.
  If required, they can check its status if things have changed.

This means for the notification during Task creation, the transaction must fail if the notification can't be delivered.
If notification fails on Task update, it should be redelivered, but the transaction may proceed.