'use client'
import CarePlanSelector from "./components/care-plan-selector";
import TaskConditionSelector from "./components/task-condition-selector";
import SelectedPatientView from "../components/selected-patient-view";
import EnrollInCpsButton from "./components/enroll-in-cps-button";
import SelectedServiceRequestView from "../components/selected-service-request-view";

export default function Home() {

  return (
    <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
      <div className="col-span-1">
        <SelectedPatientView />
        <SelectedServiceRequestView />
      </div>
      <div className="col-span-1 flex flex-col space-y-4">
        <CarePlanSelector />
        <TaskConditionSelector />
        <EnrollInCpsButton />
      </div>
    </div>
  );
}
