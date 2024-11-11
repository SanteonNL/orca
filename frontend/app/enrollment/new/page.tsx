'use client'
import EnrollInCpsButton from "./components/enroll-in-cps-button";
import EnrollmentDetailsView from "../components/enrollment-details-view";

export default function Home() {

  return (
    <>
      <EnrollmentDetailsView />
      <EnrollInCpsButton className="float-right" />
    </>
  );
}
