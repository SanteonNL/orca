import {Coding} from 'fhir/r4'
import {codingToMessage} from "@/lib/fhirUtils";
import {CircleAlert} from "lucide-react";


interface ValidationErrorsProps {
    validationErrors: Coding[]
}

export default function ValidationErrors({validationErrors}: ValidationErrorsProps) {
    const messages = codingToMessage(validationErrors);

    return (
        <div className="border rounded-lg p-4 bg-red-50 mb-4" style={{ borderColor: '#CA323B', maxWidth: 560, borderWidth: 2 }}>
            <div className="flex items-start gap-3">
                <div className="flex-shrink-0">
                    <div className="w-6 h-6 flex items-center justify-center">
                        <CircleAlert color="red"/>
                    </div>
                </div>
                <div className="flex-grow">
                    <h3 className="font-semibold mb-3" style={{ color: '#222222' }}>Er gaat iets mis</h3>
                    <div className="space-y-[8px]">
                        {messages.map((message, index) => (
                            <p key={index} style={{ color: '#222222' }}>
                                {message}
                            </p>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    )
}
