import { Coding } from 'fhir/r4'
import { codingToMessage} from "@/lib/fhirUtils";
import {CircleAlert} from "lucide-react";


interface ValidationErrorsProps {
    validationErrors: Coding[]
}

export default function ValidationErrors({ validationErrors }: ValidationErrorsProps) {
    return (
        <div className="border border-red-500 rounded-lg p-4 bg-red-50 mb-4">
            <div className="flex items-start gap-3">
                <div className="flex-shrink-0">
                    <div className="w-6 h-6 flex items-center justify-center">
                        <CircleAlert color="red" />
                    </div>
                </div>
                <div className="flex-grow">
                    <h3 className="font-semibold text-gray-900 mb-3">Er gaat iets mis</h3>
                    <div className="space-y-2">
                        <p  className="text-gray-700">
                            {codingToMessage(validationErrors)}
                        </p>
                    </div>
                </div>
            </div>
        </div>
    )
}
