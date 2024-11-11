'use client'
import EnrollInCpsButton from "./components/enroll-in-cps-button";
import EnrollmentDetailsView from "../components/enrollment-details-view";

export default function Home() {

  return (
    <div className="flex flex-col">
      <EnrollmentDetailsView />
      <EnrollInCpsButton />
    </div>
  );
}
