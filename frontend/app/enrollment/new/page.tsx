'use client'
import useEnrollment from "@/lib/store/enrollment-store";
import CarePlanSelector from "./components/care-plan-selector";
import ConditionSelector from "./components/conditions-selector";
import SelectedPatientView from "../components/selected-patient-view";
import EnrollInCpsButton from "./components/enroll-in-cps-button";

export default function Home() {

  return (
    <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
      <div className="col-span-1">
        <SelectedPatientView />
      </div>
      <div className="col-span-1 flex flex-col space-y-4">
        <CarePlanSelector />
        <ConditionSelector />
        <EnrollInCpsButton />
      </div>
    </div>
  );
}
